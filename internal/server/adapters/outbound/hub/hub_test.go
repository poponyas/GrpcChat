package server_service_hub

import (
	"context"
	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	"testing"
)

func TestMultipleSessionsAndUnsubscribe(t *testing.T) {
	h := NewHub()
	user := uuid.New()
	a, _, cancelA := h.Subscribe(user)
	b, _, cancelB := h.Subscribe(user)
	defer cancelB()
	msg := &domain.Message{ID: uuid.New(), SenderID: user}
	_ = h.SendToUsers(context.Background(), msg, []uuid.UUID{user})
	if (<-a).ID != msg.ID || (<-b).ID != msg.ID {
		t.Fatal("both terminals should receive messages, including sender echo")
	}
	cancelA()
	_ = h.SendToUsers(context.Background(), msg, []uuid.UUID{user})
	select {
	case <-a:
		t.Fatal("unsubscribed terminal received a message")
	default:
	}
	<-b
}
func TestSlowSubscriberIsDisconnected(t *testing.T) {
	h := NewHub()
	user := uuid.New()
	_, overflow, cancel := h.Subscribe(user)
	defer cancel()
	for i := 0; i < 101; i++ {
		_ = h.SendToUsers(context.Background(), &domain.Message{}, []uuid.UUID{user})
	}
	select {
	case <-overflow:
	default:
		t.Fatal("slow subscriber must be told to reload history")
	}
}
