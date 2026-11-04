package domain

import (
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestMessageUnicodeLimits(t *testing.T) {
	msg, err := NewMessage(uuid.New(), uuid.New(), "  "+strings.Repeat("я", MaxMessageLength)+"  ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(msg.Content, " ") {
		t.Fatal("message was not trimmed")
	}
	if _, err = NewMessage(uuid.New(), uuid.New(), strings.Repeat("я", MaxMessageLength+1)); err == nil {
		t.Fatal("oversized Unicode message accepted")
	}
	if _, err = NewMessage(uuid.New(), uuid.New(), " \n\t"); err == nil {
		t.Fatal("blank message accepted")
	}
}
func TestChatRules(t *testing.T) {
	title := "группа"
	members := map[uuid.UUID]MemberType{uuid.New(): ChatOwner}
	if _, err := NewChat(ChatTypeGroup, &title, members); err != nil {
		t.Fatal(err)
	}
	if _, err := NewChat(ChatTypeDirect, nil, members); err == nil {
		t.Fatal("one-person direct conversation accepted")
	}
	empty := " "
	if _, err := NewChat(ChatTypeGroup, &empty, members); err == nil {
		t.Fatal("empty group name accepted")
	}
}
