// Package cli implements a line-oriented terminal chat. All UI state belongs to
// one event loop; background goroutines only emit input or network events.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"

	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const Help = `Команды:
  /chats                       мои группы и диалоги; номера для /open
  /open <номер|имя|ID>          переключиться на чат и загрузить историю
  /group <название>            создать группу и открыть её
  /users [имя]                 список пользователей или поиск
  /dm <@имя|ID>                открыть личный диалог
  /members                     участники текущего чата
  /invite <@имя|ID>            пригласить в текущую группу (владелец)
  /remove <@имя|ID>            удалить участника (владелец)
  /leave                       выйти из текущей группы
  /history [1..100]            загрузить предыдущую страницу истории
  /help                        эта справка
  /quit                        выйти, сохранив сессию
Обычный текст отправляется в текущий чат. //текст отправляет /текст.
`

type event struct {
	message *pb.ChatMessage
	err     error
	ready   bool
}
type App struct {
	client    pb.ChatServiceClient
	users     pb.UserServiceClient
	session   Session
	out       io.Writer
	chats     []*pb.Chat
	active    *pb.Chat
	unread    map[string]int
	seen      map[string]struct{}
	seenOrder []string
	oldest    map[string]string
}

func New(conn grpc.ClientConnInterface, s Session, out io.Writer) *App {
	return &App{client: pb.NewChatServiceClient(conn), users: pb.NewUserServiceClient(conn), session: s, out: out, unread: make(map[string]int), seen: make(map[string]struct{}), oldest: make(map[string]string)}
}
func (s Session) Context(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+s.Token)
}
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s)
}
func (a *App) printf(format string, args ...any) { fmt.Fprintf(a.out, format, args...) }
func (a *App) prompt() {
	name := "выберите чат"
	if a.active != nil {
		name = safe(a.active.Name)
	}
	a.printf("[%s] > ", name)
}
func (a *App) rpc(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(a.session.Context(ctx), 10*time.Second)
}
func (a *App) refresh(ctx context.Context) error {
	var chats []*pb.Chat
	for offset := 0; ; offset += 100 {
		call, cancel := a.rpc(ctx)
		res, err := a.client.GetUserChats(call, &pb.GetUserChatsRequest{Limit: 100, Offset: int32(offset)})
		cancel()
		if err != nil {
			return err
		}
		chats = append(chats, res.Chats...)
		if len(res.Chats) < 100 {
			break
		}
	}
	a.chats = chats
	if a.active != nil {
		found := false
		for _, c := range chats {
			if c.Id == a.active.Id {
				a.active = c
				found = true
				break
			}
		}
		if !found {
			a.active = nil
		}
	}
	return nil
}
func (a *App) list() {
	if len(a.chats) == 0 {
		a.printf("Чатов пока нет. Создайте /group <название> или /dm <имя>.\n")
	}
	for i, c := range a.chats {
		kind := "группа"
		if c.IsPrivate {
			kind = "личный"
		}
		mark := " "
		if a.active != nil && a.active.Id == c.Id {
			mark = "*"
		}
		a.printf("%s %d. %s (%s, новых: %d) %s\n", mark, i+1, safe(c.Name), kind, a.unread[c.Id], c.Id)
	}
}
func (a *App) remember(id string) bool {
	if _, ok := a.seen[id]; ok {
		return false
	}
	a.seen[id] = struct{}{}
	a.seenOrder = append(a.seenOrder, id)
	if len(a.seenOrder) > 2048 {
		delete(a.seen, a.seenOrder[0])
		a.seenOrder = a.seenOrder[1:]
	}
	return true
}
func (a *App) printMessage(m *pb.ChatMessage) {
	name := m.SenderName
	if name == "" {
		name = m.SenderId
	}
	stamp := "--:--"
	if m.CreatedAt != nil {
		stamp = m.CreatedAt.AsTime().Local().Format("15:04:05")
	}
	a.printf("%s @%s: %s\n", stamp, safe(name), safe(m.Content))
}
func (a *App) incoming(ctx context.Context, m *pb.ChatMessage) {
	if !a.remember(m.Id) {
		return
	}
	if a.active != nil && a.active.Id == m.ChatId {
		a.printMessage(m)
		return
	}
	// Resolve newly invited groups / newly created direct conversations on arrival.
	var title string
	for _, c := range a.chats {
		if c.Id == m.ChatId {
			title = c.Name
		}
	}
	if title == "" {
		_ = a.refresh(ctx)
		for _, c := range a.chats {
			if c.Id == m.ChatId {
				title = c.Name
			}
		}
	}
	if title == "" {
		title = m.ChatId
	}
	a.unread[m.ChatId]++
	a.printf("Новое в %s от @%s (непрочитано: %d)\n", safe(title), safe(m.SenderName), a.unread[m.ChatId])
}
func (a *App) history(ctx context.Context, limit int, older bool) error {
	if a.active == nil {
		return errors.New("сначала откройте чат: /open <номер>")
	}
	req := &pb.GetChatHistoryRequest{ChatId: a.active.Id, Limit: int32(limit)}
	if older {
		if id := a.oldest[a.active.Id]; id != "" {
			req.BeforeMessageId = &id
		}
	}
	call, cancel := a.rpc(ctx)
	defer cancel()
	res, err := a.client.GetChatHistory(call, req)
	if err != nil {
		return err
	}
	if len(res.Messages) == 0 {
		a.printf("Больше сообщений нет.\n")
		return nil
	}
	a.oldest[a.active.Id] = res.Messages[0].Id
	for _, m := range res.Messages {
		a.remember(m.Id)
		a.printMessage(m)
	}
	return nil
}
func (a *App) open(ctx context.Context, c *pb.Chat) error {
	a.active = c
	a.unread[c.Id] = 0
	delete(a.oldest, c.Id)
	a.printf("Открыт %s\n", safe(c.Name))
	return a.history(ctx, 30, false)
}
func (a *App) findChat(query string) (*pb.Chat, error) {
	if n, err := strconv.Atoi(query); err == nil && n >= 1 && n <= len(a.chats) {
		return a.chats[n-1], nil
	}
	var found *pb.Chat
	for _, c := range a.chats {
		if c.Id == query || strings.EqualFold(c.Name, query) {
			if found != nil {
				return nil, errors.New("несколько чатов с таким именем; используйте номер или ID")
			}
			found = c
		}
	}
	if found == nil {
		return nil, errors.New("чат не найден; обновите /chats и используйте номер или ID")
	}
	return found, nil
}
func (a *App) allUsers(ctx context.Context) ([]*pb.UserProfile, error) {
	var users []*pb.UserProfile
	for offset := 0; ; offset += 100 {
		call, cancel := a.rpc(ctx)
		res, err := a.users.GetProfiles(call, &pb.GetProfilesInput{Limit: 100, Offset: int32(offset)})
		cancel()
		if err != nil {
			return nil, err
		}
		users = append(users, res.Profiles...)
		if len(res.Profiles) < 100 {
			break
		}
	}
	return users, nil
}
func (a *App) resolveUser(ctx context.Context, query string) (*pb.UserProfile, error) {
	users, err := a.allUsers(ctx)
	if err != nil {
		return nil, err
	}
	query = strings.TrimPrefix(query, "@")
	var found *pb.UserProfile
	for _, u := range users {
		if u.Id == query || strings.EqualFold(u.Name, query) {
			if found != nil {
				return nil, errors.New("имя неоднозначно; используйте ID из /users")
			}
			found = u
		}
	}
	if found == nil {
		return nil, errors.New("пользователь не найден: /users")
	}
	return found, nil
}
func (a *App) command(ctx context.Context, line string) (bool, error) {
	cmd, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	switch cmd {
	case "/quit", "/exit":
		return true, nil
	case "/help":
		a.printf("%s", Help)
	case "/chats":
		if err := a.refresh(ctx); err != nil {
			return false, err
		}
		a.list()
	case "/open":
		c, err := a.findChat(arg)
		if err != nil {
			return false, err
		}
		return false, a.open(ctx, c)
	case "/group", "/dm":
		req := &pb.CreateChatRequest{Name: arg, IsPrivate: cmd == "/dm"}
		if cmd == "/dm" {
			u, err := a.resolveUser(ctx, arg)
			if err != nil {
				return false, err
			}
			req.MemberIds = []string{u.Id}
		}
		call, cancel := a.rpc(ctx)
		res, err := a.client.CreateChat(call, req)
		cancel()
		if err != nil {
			return false, err
		}
		if err = a.refresh(ctx); err != nil {
			return false, err
		}
		return false, a.open(ctx, res.Chat)
	case "/users":
		users, err := a.allUsers(ctx)
		if err != nil {
			return false, err
		}
		for _, u := range users {
			if arg == "" || strings.Contains(strings.ToLower(u.Name), strings.ToLower(strings.TrimPrefix(arg, "@"))) {
				a.printf("@%s — %s — %s\n", safe(u.Name), safe(u.Fullname), u.Id)
			}
		}
	case "/members":
		if a.active == nil {
			return false, errors.New("сначала выберите чат")
		}
		call, cancel := a.rpc(ctx)
		res, err := a.client.GetMembers(call, &pb.GetMembersRequest{ChatId: a.active.Id})
		cancel()
		if err != nil {
			return false, err
		}
		for _, m := range res.Members {
			a.printf("@%s (%s) %s\n", safe(m.Name), safe(m.Role), m.UserId)
		}
	case "/invite", "/remove", "/leave":
		if a.active == nil {
			return false, errors.New("сначала выберите группу")
		}
		user := a.session.UserID
		if cmd != "/leave" {
			u, err := a.resolveUser(ctx, arg)
			if err != nil {
				return false, err
			}
			user = u.Id
		}
		call, cancel := a.rpc(ctx)
		var err error
		if cmd == "/invite" {
			_, err = a.client.AddMember(call, &pb.AddMemberRequest{ChatId: a.active.Id, UserId: user})
		} else {
			_, err = a.client.RemoveMember(call, &pb.RemoveMemberRequest{ChatId: a.active.Id, UserId: user})
		}
		cancel()
		if err != nil {
			return false, err
		}
		a.printf("Участники обновлены.\n")
		return false, a.refresh(ctx)
	case "/history":
		limit := 30
		if arg != "" {
			n, err := strconv.Atoi(arg)
			if err != nil || n < 1 || n > 100 {
				return false, errors.New("/history: размер страницы от 1 до 100")
			}
			limit = n
		}
		return false, a.history(ctx, limit, true)
	default:
		return false, errors.New("неизвестная команда; /help")
	}
	return false, nil
}
func (a *App) send(ctx context.Context, text string) error {
	if a.active == nil {
		return errors.New("сначала выберите чат: /chats, /open <номер>")
	}
	call, cancel := a.rpc(ctx)
	defer cancel()
	m, err := a.client.SendMessage(call, &pb.SendMessageRequest{ChatId: a.active.Id, Content: text})
	if err != nil {
		return err
	}
	if a.remember(m.Id) {
		a.printMessage(m)
	}
	return nil
}
func (a *App) listen(ctx context.Context, events chan<- event) {
	emit := func(e event) bool {
		select {
		case events <- e:
			return true
		case <-ctx.Done():
			return false
		}
	}
	delay := time.Second
	for ctx.Err() == nil {
		call, cancel := context.WithCancel(a.session.Context(ctx))
		stream, err := a.client.Connect(call)
		if err == nil {
			_, err = stream.Header()
		}
		if err == nil {
			delay = time.Second
			if !emit(event{ready: true}) {
				cancel()
				return
			}
			for {
				m, recvErr := stream.Recv()
				if recvErr != nil {
					err = recvErr
					break
				}
				if !emit(event{message: m}) {
					cancel()
					return
				}
			}
		}
		cancel()
		if ctx.Err() != nil {
			return
		}
		if !emit(event{err: err}) {
			return
		}
		if status.Code(err) == codes.Unauthenticated || status.Code(err) == codes.PermissionDenied {
			return
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if delay < 5*time.Second {
			delay *= 2
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
		}
	}
}
func (a *App) Run(ctx context.Context, in io.Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := a.refresh(ctx); err != nil {
		return err
	}
	a.printf("GrpcChat · @%s\n%s\n", safe(a.session.Name), Help)
	a.list()
	events := make(chan event, 100)
	go a.listen(ctx, events)
	lines := make(chan string)
	inputErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 4096), 64*1024)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		inputErr <- scanner.Err()
	}()
	a.prompt()
	connected := false
	wasConnected := false
	for {
		select {
		case <-ctx.Done():
			a.printf("\nДо встречи.\n")
			return nil
		case err := <-inputErr:
			if err != nil {
				return err
			}
			a.printf("\nДо встречи.\n")
			return nil
		case e := <-events:
			if e.err != nil {
				connected = false
				if status.Code(e.err) == codes.Unauthenticated || status.Code(e.err) == codes.PermissionDenied {
					return errors.New("сессия истекла или отозвана; выполните chat-cli login")
				}
				a.printf("\nСвязь с сервером прервана. Переподключение…\n")
			}
			if e.ready {
				connected = true
				a.printf("\nСоединение установлено.\n")
				if wasConnected {
					if err := a.refresh(ctx); err != nil {
						a.printf("Ошибка: %s\n", safe(status.Convert(err).Message()))
					}
					if a.active != nil {
						if err := a.history(ctx, 30, false); err != nil {
							a.printf("История: %s\n", safe(status.Convert(err).Message()))
						}
					}
				}
				wasConnected = true
			}
			if e.message != nil {
				if _, known := a.seen[e.message.Id]; known {
					continue
				}
				a.printf("\n")
				a.incoming(ctx, e.message)
			}
			a.prompt()
		case line := <-lines:
			line = strings.TrimSpace(line)
			if line == "" {
				a.prompt()
				continue
			}
			var quit bool
			var err error
			if strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "//") {
				quit, err = a.command(ctx, line)
			} else {
				if !connected {
					err = errors.New("нет соединения; дождитесь переподключения")
				} else {
					text := line
					if strings.HasPrefix(text, "//") {
						text = text[1:]
					}
					err = a.send(ctx, text)
				}
			}
			if err != nil {
				a.printf("Ошибка: %s\n", safe(status.Convert(err).Message()))
			}
			if quit {
				a.printf("До встречи.\n")
				return nil
			}
			a.prompt()
		}
	}
}
