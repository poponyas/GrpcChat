package server_service_ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
)

// Outbound Port: База данных
type MessageRepository interface {
	SaveMessage(ctx context.Context, msg *domain.Message) error
	GetHistory(ctx context.Context, chatID uuid.UUID, limit int, beforeID *uuid.UUID) ([]*domain.Message, error)
}

// Outbound Port: Менеджер активных подключений (Hub)
// Ядро использует его, чтобы рассылать сообщения онлайн-пользователям
// Outbound Port: Менеджер активных подключений
type ChatNotifier interface {
	SendToUsers(ctx context.Context, msg *domain.Message, recipientIDs []uuid.UUID) error
	RegisterClient(userID uuid.UUID, clientChan chan<- *domain.Message)
	UnregisterClient(userID uuid.UUID)
}

// Inbound Port: Бизнес-логика чата (UseCase)
type ChatRepository interface {
	SaveChat(ctx context.Context, chat *domain.Chat) error
	GetChatByID(ctx context.Context, chatID uuid.UUID) (*domain.Chat, error)
}
