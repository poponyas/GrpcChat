package server_service_hub

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	"sync"
)

type subscription struct {
	messages chan *domain.Message
	overflow chan struct{}
	failed   bool
}
type Hub struct {
	mu      sync.Mutex
	clients map[uuid.UUID]map[*subscription]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[uuid.UUID]map[*subscription]struct{})} }

// Each connection has its own subscription, including multiple terminals of one user.
func (h *Hub) Subscribe(user uuid.UUID) (<-chan *domain.Message, <-chan struct{}, func()) {
	s := &subscription{messages: make(chan *domain.Message, 100), overflow: make(chan struct{})}
	h.mu.Lock()
	if h.clients[user] == nil {
		h.clients[user] = make(map[*subscription]struct{})
	}
	h.clients[user][s] = struct{}{}
	h.mu.Unlock()
	return s.messages, s.overflow, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.clients[user], s)
		if len(h.clients[user]) == 0 {
			delete(h.clients, user)
		}
	}
}
func (h *Hub) SendToUsers(ctx context.Context, msg *domain.Message, users []uuid.UUID) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, user := range users {
		for s := range h.clients[user] {
			if s.failed {
				continue
			}
			select {
			case s.messages <- msg:
			default:
				s.failed = true
				close(s.overflow)
			}
		}
	}
	return nil
}
