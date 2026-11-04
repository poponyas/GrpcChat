package adapters_inbound_grpc

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	ports "github.com/poponyas/GrpcChat/internal/server/ports"
	service "github.com/poponyas/GrpcChat/internal/server/service"
	pkgcontext "github.com/poponyas/GrpcChat/pkg/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ChatHandler struct {
	pb.UnimplementedChatServiceServer
	service *service.ChatService
	hub     ports.ChatNotifier
}

func NewChatHandler(s *service.ChatService, h ports.ChatNotifier) *ChatHandler {
	return &ChatHandler{service: s, hub: h}
}
func rpcError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrAccessDenied):
		return status.Error(codes.PermissionDenied, "you are not allowed to access this chat or manage its members")
	case errors.Is(err, service.ErrChatNotFound):
		return status.Error(codes.NotFound, "chat not found")
	case errors.Is(err, service.ErrUserNotFound):
		return status.Error(codes.NotFound, "user not found")
	case errors.Is(err, service.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")
	default:
		return status.Error(codes.Internal, "chat operation failed")
	}
}
func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Error(codes.InvalidArgument, "expected a non-empty UUID")
	}
	return id, nil
}
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, ok := pkgcontext.GetUserID(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "login required")
	}
	return id, nil
}
func messagePB(m *domain.Message) *pb.ChatMessage {
	return &pb.ChatMessage{Id: m.ID.String(), ChatId: m.ChatID.String(), SenderId: m.SenderID.String(), SenderName: m.SenderName, Content: m.Content, CreatedAt: timestamppb.New(m.CreatedAt)}
}
func membersPB(c *domain.Chat) []*pb.ChatMember {
	members := make([]*pb.ChatMember, 0, len(c.Members))
	for id, role := range c.Members {
		members = append(members, &pb.ChatMember{UserId: id.String(), Name: c.MemberNames[id], Role: string(role)})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	return members
}
func chatPB(c *domain.Chat, user uuid.UUID) *pb.Chat {
	name := ""
	if c.Title != nil {
		name = *c.Title
	}
	if c.Type == domain.ChatTypeDirect {
		for id, n := range c.MemberNames {
			if id != user {
				name = "@" + n
			}
		}
	}
	return &pb.Chat{Id: c.ID.String(), Name: name, IsPrivate: c.Type == domain.ChatTypeDirect, CreatedAt: timestamppb.New(c.CreatedAt), Members: membersPB(c)}
}
func (h *ChatHandler) CreateChat(ctx context.Context, in *pb.CreateChatRequest) (*pb.CreateChatResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if len(in.MemberIds) > 99 {
		return nil, status.Error(codes.InvalidArgument, "at most 99 initial members")
	}
	members := map[uuid.UUID]domain.MemberType{user: domain.ChatOwner}
	for _, raw := range in.MemberIds {
		id, err := parseID(raw)
		if err != nil {
			return nil, err
		}
		if id != user {
			members[id] = domain.ChatParticipant
		}
	}
	kind := domain.ChatTypeGroup
	name := strings.TrimSpace(in.Name)
	if in.IsPrivate {
		kind = domain.ChatTypeDirect
		if len(members) != 2 {
			return nil, status.Error(codes.InvalidArgument, "direct chat requires one other user")
		}
		for id := range members {
			members[id] = domain.ChatParticipant
		}
	}
	c, err := h.service.CreateChat(ctx, kind, &name, members)
	if err != nil {
		return nil, rpcError(err)
	}
	c, err = h.service.GetChat(ctx, user, c.ID)
	if err != nil {
		return nil, rpcError(err)
	}
	return &pb.CreateChatResponse{Chat: chatPB(c, user)}, nil
}
func (h *ChatHandler) GetUserChats(ctx context.Context, in *pb.GetUserChatsRequest) (*pb.GetUserChatsResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId != "" && in.UserId != user.String() {
		return nil, status.Error(codes.PermissionDenied, "can only list your own chats")
	}
	chats, err := h.service.ListChats(ctx, user, int(in.Limit), int(in.Offset))
	if err != nil {
		return nil, rpcError(err)
	}
	out := &pb.GetUserChatsResponse{}
	for _, c := range chats {
		out.Chats = append(out.Chats, chatPB(c, user))
	}
	return out, nil
}
func (h *ChatHandler) GetChatHistory(ctx context.Context, in *pb.GetChatHistoryRequest) (*pb.GetChatHistoryResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	chat, err := parseID(in.ChatId)
	if err != nil {
		return nil, err
	}
	var before *uuid.UUID
	if in.BeforeMessageId != nil {
		id, err := parseID(*in.BeforeMessageId)
		if err != nil {
			return nil, err
		}
		before = &id
	}
	msgs, err := h.service.GetHistory(ctx, user, chat, int(in.Limit), before)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &pb.GetChatHistoryResponse{}
	for _, m := range msgs {
		out.Messages = append(out.Messages, messagePB(m))
	}
	return out, nil
}
func (h *ChatHandler) GetMembers(ctx context.Context, in *pb.GetMembersRequest) (*pb.GetMembersResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(in.ChatId)
	if err != nil {
		return nil, err
	}
	c, err := h.service.GetChat(ctx, user, id)
	if err != nil {
		return nil, rpcError(err)
	}
	return &pb.GetMembersResponse{Members: membersPB(c)}, nil
}
func (h *ChatHandler) AddMember(ctx context.Context, in *pb.AddMemberRequest) (*emptypb.Empty, error) {
	return h.changeMember(ctx, in.ChatId, in.UserId, false)
}
func (h *ChatHandler) RemoveMember(ctx context.Context, in *pb.RemoveMemberRequest) (*emptypb.Empty, error) {
	return h.changeMember(ctx, in.ChatId, in.UserId, true)
}
func (h *ChatHandler) changeMember(ctx context.Context, chatRaw, userRaw string, remove bool) (*emptypb.Empty, error) {
	actor, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	chat, err := parseID(chatRaw)
	if err != nil {
		return nil, err
	}
	user, err := parseID(userRaw)
	if err != nil {
		return nil, err
	}
	if err = h.service.ChangeMember(ctx, actor, chat, user, remove); err != nil {
		return nil, rpcError(err)
	}
	return &emptypb.Empty{}, nil
}
func (h *ChatHandler) SendMessage(ctx context.Context, in *pb.SendMessageRequest) (*pb.ChatMessage, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	chat, err := parseID(in.ChatId)
	if err != nil {
		return nil, err
	}
	m, err := h.service.SendMessage(ctx, user, chat, in.Content)
	if err != nil {
		return nil, rpcError(err)
	}
	return messagePB(m), nil
}
func (h *ChatHandler) Connect(stream grpc.BidiStreamingServer[pb.ChatMessage, pb.ChatMessage]) error {
	user, err := currentUser(stream.Context())
	if err != nil {
		return err
	}
	messages, overflow, cancel := h.hub.Subscribe(user)
	defer cancel()
	// The client can wait for headers before sending its first message.
	if err := stream.SendHeader(metadata.Pairs("chat-ready", "true")); err != nil {
		return err
	}
	received := make(chan *pb.ChatMessage)
	recvErr := make(chan error, 1)
	go func() {
		for {
			m, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			select {
			case received <- m:
			case <-stream.Context().Done():
				return
			}
		}
	}()
	// Only this goroutine calls Send. Recv exits when the RPC returns/cancels.
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-overflow:
			return status.Error(codes.ResourceExhausted, "client too slow; reconnect and reload history")
		case err := <-recvErr:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case in := <-received:
			if _, err := h.SendMessage(stream.Context(), &pb.SendMessageRequest{ChatId: in.ChatId, Content: in.Content}); err != nil {
				return err
			}
		case msg := <-messages:
			// Revoked members do not receive queued events after removal.
			if _, err := h.service.GetChat(stream.Context(), user, msg.ChatID); err != nil {
				if errors.Is(err, service.ErrAccessDenied) || errors.Is(err, service.ErrChatNotFound) {
					continue
				}
				return rpcError(err)
			}
			if err := stream.Send(messagePB(msg)); err != nil {
				return err
			}
		}
	}
}
