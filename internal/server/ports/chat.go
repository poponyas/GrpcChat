package server_service_ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
)

type MessageRepository interface {
	SaveMessage(context.Context, *domain.Message) error
	GetHistory(context.Context, uuid.UUID, uuid.UUID, int, *uuid.UUID) ([]*domain.Message, error)
}
type ChatRepository interface {
	SaveChat(context.Context, *domain.Chat) error
	GetChatByID(context.Context, uuid.UUID) (*domain.Chat, error)
	ListChats(context.Context, uuid.UUID, int, int) ([]*domain.Chat, error)
	ChangeMember(ctx context.Context, actor, chat, user uuid.UUID, remove bool) error
}
type ChatNotifier interface {
	SendToUsers(context.Context, *domain.Message, []uuid.UUID) error
	Subscribe(uuid.UUID) (messages <-chan *domain.Message, overflow <-chan struct{}, cancel func())
}
