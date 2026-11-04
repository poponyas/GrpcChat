package server_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	server_service_ports "github.com/poponyas/GrpcChat/internal/server/ports"
)

var (
	ErrAccessDenied = errors.New("user is not a member of this chat")
	ErrChatNotFound = errors.New("chat not found")
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidInput = errors.New("invalid input")
)

type ChatService struct {
	chatRepo    server_service_ports.ChatRepository
	messageRepo server_service_ports.MessageRepository
	notifier    server_service_ports.ChatNotifier
}

func NewChatService(
	chatRepo server_service_ports.ChatRepository,
	messageRepo server_service_ports.MessageRepository,
	notifier server_service_ports.ChatNotifier,
) *ChatService {
	return &ChatService{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
		notifier:    notifier,
	}
}

// CreateChat валидирует и сохраняет новый чат
func (s *ChatService) CreateChat(
	ctx context.Context,
	chatType domain.ChatType,
	title *string,
	members map[uuid.UUID]domain.MemberType,
) (*domain.Chat, error) {
	chat, err := domain.NewChat(chatType, title, members)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if err := s.chatRepo.SaveChat(ctx, chat); err != nil {
		return nil, fmt.Errorf("failed to save chat: %w", err)
	}

	return chat, nil
}

// SendMessage проверяет права пользователя, валидирует текст, сохраняет в БД и рассылает через Hub
func (s *ChatService) SendMessage(
	ctx context.Context,
	senderID uuid.UUID,
	chatID uuid.UUID,
	content string,
) (*domain.Message, error) {
	// 1. Проверяем существование чата
	chat, err := s.chatRepo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}
	if chat == nil {
		return nil, ErrChatNotFound
	}

	// 2. Проверяем, что отправитель состоит в этом чате (O(1) благодаря map)
	if !chat.IsMember(senderID) {
		return nil, ErrAccessDenied
	}

	// 3. Создаем и валидируем доменную модель сообщения
	msg, err := domain.NewMessage(chatID, senderID, content)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// 4. Персистим сообщение в базу данных
	if err := s.messageRepo.SaveMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to persist message: %w", err)
	}

	// 5. Собираем списки получателей и передаем в Hub для рассылки в реальном времени
	recipientIDs := make([]uuid.UUID, 0, len(chat.Members))
	for memberID := range chat.Members {
		recipientIDs = append(recipientIDs, memberID)
	}

	// Ошибку рассылки только логируем, так как сообщение уже успешно сохранилось в БД
	_ = s.notifier.SendToUsers(ctx, msg, recipientIDs)

	return msg, nil
}

// GetHistory возвращает историю сообщений чата с проверкой прав доступа
func (s *ChatService) GetHistory(
	ctx context.Context,
	userID uuid.UUID,
	chatID uuid.UUID,
	limit int,
	beforeID *uuid.UUID,
) ([]*domain.Message, error) {
	chat, err := s.chatRepo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}
	if chat == nil {
		return nil, ErrChatNotFound
	}

	if !chat.IsMember(userID) {
		return nil, ErrAccessDenied
	}

	if limit < 0 {
		return nil, ErrInvalidInput
	}
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.messageRepo.GetHistory(ctx, chatID, userID, limit, beforeID)
}

func (s *ChatService) GetChat(ctx context.Context, user, chatID uuid.UUID) (*domain.Chat, error) {
	chat, err := s.chatRepo.GetChatByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if chat == nil {
		return nil, ErrChatNotFound
	}
	if !chat.IsMember(user) {
		return nil, ErrAccessDenied
	}
	return chat, nil
}
func (s *ChatService) ListChats(ctx context.Context, user uuid.UUID, limit, offset int) ([]*domain.Chat, error) {
	if limit < 0 || offset < 0 {
		return nil, ErrInvalidInput
	}
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.chatRepo.ListChats(ctx, user, limit, offset)
}
func (s *ChatService) ChangeMember(ctx context.Context, actor, chat, user uuid.UUID, remove bool) error {
	return s.chatRepo.ChangeMember(ctx, actor, chat, user, remove)
}
