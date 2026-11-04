package integration

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	pooladapter "github.com/poponyas/GrpcChat/core/repository/postgres/pgx"
	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"github.com/poponyas/GrpcChat/internal/auth"
	"github.com/poponyas/GrpcChat/internal/cli"
	profiles "github.com/poponyas/GrpcChat/internal/client/adapters"
	usergrpc "github.com/poponyas/GrpcChat/internal/client/controllers/grpc/v1"
	users "github.com/poponyas/GrpcChat/internal/client/usecase"
	chatgrpc "github.com/poponyas/GrpcChat/internal/server/adapters/inbound/grpc"
	hub "github.com/poponyas/GrpcChat/internal/server/adapters/outbound/hub"
	repository "github.com/poponyas/GrpcChat/internal/server/adapters/outbound/postgres"
	service "github.com/poponyas/GrpcChat/internal/server/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func setup(t *testing.T) (context.Context, *grpc.ClientConn, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a dedicated database ending in _test")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") {
		t.Fatal("integration tests require database name ending in _test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = pool.Exec(ctx, "DROP SCHEMA IF EXISTS chat CASCADE"); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migration %s: %v", file, err)
		}
		if filepath.Base(file) == "000001_init.up.sql" {
			_, err = pool.Exec(ctx, `INSERT INTO chat.users(id,name,fullname) VALUES('10000000-0000-0000-0000-000000000001','legacy','Legacy profile');
            INSERT INTO chat.chats(id,name) VALUES('10000000-0000-0000-0000-000000000002','Legacy group');
            INSERT INTO chat.chat_members(chat_id,user_id) VALUES('10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000001');
            INSERT INTO chat.messages(id,chat_id,sender_id,content) VALUES('10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000001','Legacy message')`)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var owner, content string
	if err := pool.QueryRow(ctx, `SELECT m.role,s.content FROM chat.chat_members m JOIN chat.messages s ON s.chat_id=m.chat_id WHERE m.chat_id='10000000-0000-0000-0000-000000000002'`).Scan(&owner, &content); err != nil || owner != "owner" || content != "Legacy message" {
		t.Fatalf("legacy migration: role=%s content=%s err=%v", owner, content, err)
	}
	accounts := auth.New(pool)
	repo := repository.New(pool)
	events := hub.NewHub()
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(accounts.UnaryInterceptor()), grpc.ChainStreamInterceptor(accounts.StreamInterceptor()))
	pb.RegisterAuthServiceServer(srv, accounts)
	pb.RegisterChatServiceServer(srv, chatgrpc.NewChatHandler(service.NewChatService(repo, repo, events), events))
	adapter := &pooladapter.Pool{}
	// The public constructor handles DSN configuration; tests already own the pool.
	adapter.Pool = pool
	pb.RegisterUserServiceServer(srv, usergrpc.NewTransportGRPC(users.NewUseCasegRPC(profiles.NewUserRepository(&testPool{Pool: adapter}))))
	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return ctx, conn, pool
}

// Override the operation timeout while retaining production SQL adapters.
type testPool struct{ *pooladapter.Pool }

func (*testPool) OperationTimeout() time.Duration { return 5 * time.Second }
func expectCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("code=%v want=%v error=%v", status.Code(err), want, err)
	}
}
func TestChatEndToEnd(t *testing.T) {
	ctx, conn, pool := setup(t)
	accounts := pb.NewAuthServiceClient(conn)
	chats := pb.NewChatServiceClient(conn)
	register := func(name string) *pb.AuthResponse {
		t.Helper()
		res, err := accounts.Register(ctx, &pb.RegisterRequest{Name: name, Password: "test-password-123"})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	alice, bob, eve := register("alice"), register("bob"), register("eve")
	userCtx := func(u *pb.AuthResponse) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+u.Token)
	}
	a, b, e := userCtx(alice), userCtx(bob), userCtx(eve)
	_, err := accounts.Register(ctx, &pb.RegisterRequest{Name: "ALICE", Password: "test-password-123"})
	expectCode(t, err, codes.AlreadyExists)
	_, err = accounts.Login(ctx, &pb.LoginRequest{Name: "alice", Password: "wrong"})
	expectCode(t, err, codes.Unauthenticated)
	login, err := accounts.Login(ctx, &pb.LoginRequest{Name: "ALICE", Password: "test-password-123"})
	if err != nil {
		t.Fatal(err)
	}
	if login.Profile.Id != alice.Profile.Id {
		t.Fatal("login changed identity")
	}
	_, err = chats.GetUserChats(ctx, &pb.GetUserChatsRequest{})
	expectCode(t, err, codes.Unauthenticated)
	group, err := chats.CreateChat(a, &pb.CreateChatRequest{Name: "Разработка", MemberIds: []string{bob.Profile.Id}})
	if err != nil {
		t.Fatal(err)
	}
	id := group.Chat.Id
	_, err = chats.GetUserChats(b, &pb.GetUserChatsRequest{UserId: alice.Profile.Id})
	expectCode(t, err, codes.PermissionDenied)
	_, err = chats.GetChatHistory(e, &pb.GetChatHistoryRequest{ChatId: id})
	expectCode(t, err, codes.PermissionDenied)
	_, err = chats.AddMember(b, &pb.AddMemberRequest{ChatId: id, UserId: eve.Profile.Id})
	expectCode(t, err, codes.PermissionDenied)
	streams := make([]grpc.BidiStreamingClient[pb.ChatMessage, pb.ChatMessage], 0, 2)
	for i := 0; i < 2; i++ {
		stream, err := chats.Connect(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = stream.Header(); err != nil {
			t.Fatal(err)
		}
		streams = append(streams, stream)
	}
	var sent []*pb.ChatMessage
	for _, text := range []string{"Привет, команда!", "Второе сообщение", "Третье сообщение"} {
		m, err := chats.SendMessage(a, &pb.SendMessageRequest{ChatId: id, Content: text})
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, m)
		for _, stream := range streams {
			got, err := stream.Recv()
			if err != nil {
				t.Fatal(err)
			}
			if got.Id != m.Id || got.SenderName != "alice" {
				t.Fatalf("unexpected live event: %v", got)
			}
		}
	}
	page, err := chats.GetChatHistory(b, &pb.GetChatHistoryRequest{ChatId: id, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 || page.Messages[0].Id != sent[1].Id {
		t.Fatalf("unexpected first page: %v", page)
	}
	before := page.Messages[0].Id
	page, err = chats.GetChatHistory(b, &pb.GetChatHistoryRequest{ChatId: id, Limit: 2, BeforeMessageId: &before})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].Id != sent[0].Id {
		t.Fatal("history cursor skipped or repeated a message")
	}
	spoof, err := chats.Connect(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = spoof.Header(); err != nil {
		t.Fatal(err)
	}
	if err = spoof.Send(&pb.ChatMessage{ChatId: id, SenderId: eve.Profile.Id, Content: "Stream message"}); err != nil {
		t.Fatal(err)
	}
	echoed, err := spoof.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if echoed.SenderId != alice.Profile.Id {
		t.Fatal("stream accepted a forged sender")
	}
	_ = spoof.CloseSend()
	for _, stream := range streams {
		if _, err = stream.Recv(); err != nil {
			t.Fatal(err)
		}
	}
	_, err = chats.SendMessage(a, &pb.SendMessageRequest{ChatId: id, Content: "   "})
	expectCode(t, err, codes.InvalidArgument)
	_, err = chats.GetChatHistory(a, &pb.GetChatHistoryRequest{ChatId: "not-a-uuid"})
	expectCode(t, err, codes.InvalidArgument)
	_, err = chats.GetUserChats(a, &pb.GetUserChatsRequest{Limit: -1})
	expectCode(t, err, codes.InvalidArgument)
	_, err = chats.AddMember(a, &pb.AddMemberRequest{ChatId: id, UserId: uuid.NewString()})
	expectCode(t, err, codes.NotFound)
	_, err = chats.RemoveMember(a, &pb.RemoveMemberRequest{ChatId: id, UserId: bob.Profile.Id})
	if err != nil {
		t.Fatal(err)
	}
	_, err = chats.SendMessage(b, &pb.SendMessageRequest{ChatId: id, Content: "cannot send"})
	expectCode(t, err, codes.PermissionDenied)
	_, err = chats.GetChatHistory(b, &pb.GetChatHistoryRequest{ChatId: id})
	expectCode(t, err, codes.PermissionDenied)
	// Concurrent creation in both directions must produce one persistent DM.
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			call, peer := a, bob.Profile.Id
			if i%2 == 1 {
				call, peer = b, alice.Profile.Id
			}
			res, err := chats.CreateChat(call, &pb.CreateChatRequest{IsPrivate: true, MemberIds: []string{peer}})
			if err != nil {
				errs <- err
				return
			}
			ids <- res.Chat.Id
		}(i)
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	direct := ""
	for id := range ids {
		if direct != "" && direct != id {
			t.Fatal("duplicate DM")
		}
		direct = id
	}
	_, err = chats.AddMember(a, &pb.AddMemberRequest{ChatId: direct, UserId: eve.Profile.Id})
	expectCode(t, err, codes.InvalidArgument)
	_, err = chats.GetChatHistory(a, &pb.GetChatHistoryRequest{ChatId: direct, BeforeMessageId: &before})
	expectCode(t, err, codes.InvalidArgument)
	_, err = chats.CreateChat(a, &pb.CreateChatRequest{IsPrivate: true, MemberIds: []string{alice.Profile.Id}})
	expectCode(t, err, codes.InvalidArgument)
	// The owner can delete an empty group by leaving it.
	_, err = chats.RemoveMember(a, &pb.RemoveMemberRequest{ChatId: id, UserId: alice.Profile.Id})
	if err != nil {
		t.Fatal(err)
	}
	_, err = chats.GetChatHistory(a, &pb.GetChatHistoryRequest{ChatId: id})
	expectCode(t, err, codes.NotFound)
	_, err = accounts.Logout(e, &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = chats.GetUserChats(e, &pb.GetUserChatsRequest{})
	expectCode(t, err, codes.Unauthenticated)
	_, err = pool.Exec(ctx, `UPDATE chat.sessions SET expires_at=now()-interval '1 second' WHERE user_id=$1`, bob.Profile.Id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = chats.GetUserChats(b, &pb.GetUserChatsRequest{})
	expectCode(t, err, codes.Unauthenticated)
	// A fresh service instance can still read data: state lives in PostgreSQL.
	repo := repository.New(pool)
	c, err := repo.GetChatByID(ctx, uuid.MustParse(direct))
	if err != nil || len(c.Members) != 2 {
		t.Fatalf("persistent DM: %v %v", c, err)
	}
	// Exercise actual CLI parsing, navigation, invitations and history via gRPC.
	input := "/users\n/group Клиентская группа\n/invite eve\n/members\n/dm eve\n/chats\n/open Клиентская группа\n/history\n/quit\n"
	output := new(strings.Builder)
	app := cli.New(conn, cli.Session{Token: alice.Token, UserID: alice.Profile.Id, Name: "alice"}, output)
	if err := app.Run(ctx, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@eve", "Клиентская группа", "Участники обновлены.", "Открыт @eve", "До встречи."} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("CLI output missing %q:\n%s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "Ошибка:") {
		t.Fatalf("CLI command failed:\n%s", output.String())
	}
	// Exercise sending from the CLI only after the live subscription is ready.
	reader, writer := io.Pipe()
	ready := make(chan struct{})
	liveOutput := &readyOutput{ready: ready}
	done := make(chan error, 1)
	go func() {
		done <- cli.New(conn, cli.Session{Token: alice.Token, UserID: alice.Profile.Id, Name: "alice"}, liveOutput).Run(ctx, reader)
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("CLI did not connect")
	}
	if _, err := io.WriteString(writer, "/dm eve\nСообщение из CLI\n//команда как текст\n/quit\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(liveOutput.String(), "@alice: Сообщение из CLI") || !strings.Contains(liveOutput.String(), "@alice: /команда как текст") {
		t.Fatalf("CLI send failed:\n%s", liveOutput.String())
	}
	// Close both subscriptions cleanly.
	for _, stream := range streams {
		_ = stream.CloseSend()
	}
}

type readyOutput struct {
	strings.Builder
	ready chan struct{}
	once  sync.Once
}

func (w *readyOutput) Write(p []byte) (int, error) {
	n, err := w.Builder.Write(p)
	if strings.Contains(string(p), "Соединение установлено.") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}
