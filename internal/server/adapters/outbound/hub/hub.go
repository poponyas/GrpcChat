package server_service_hub

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]chan<- *domain.Message
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[uuid.UUID]chan<- *domain.Message),
	}
}

// RegisterClient регистрирует канал сообщений для подключившегося пользователя.
func (h *Hub) RegisterClient(userID uuid.UUID, clientChan chan<- *domain.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[userID] = clientChan
}

// UnregisterClient удаляет подключение пользователя при разрыве gRPC-стрима.
func (h *Hub) UnregisterClient(userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, userID)
}

// SendToUsers рассылает сообщение конкретным пользователям (участникам чата), если они онлайн.
func (h *Hub) SendToUsers(ctx context.Context, msg *domain.Message, recipientIDs []uuid.UUID) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range recipientIDs {
		// Не отправляем сообщение самому автору
		if userID == msg.SenderID {
			continue
		}

		if clientChan, ok := h.clients[userID]; ok {
			// Неблокирующая отправка:
			// Если клиент медленно читает и буфер канала переполнен,
			// мы не заблокируем сервер и остальных пользователей в цикле.
			select {
			case clientChan <- msg:
			default:
				// Опционально: здесь можно залогировать drop сообщения или переполнение буфера
			}
		}
	}

	return nil
}
