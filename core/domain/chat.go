package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const MaxMessageLength = 4096

type ChatType string
type MemberType string

const (
	ChatTypeDirect ChatType = "direct"
	ChatTypeGroup  ChatType = "group"
)

const (
	ChatOwner       MemberType = "owner"
	ChatAdmin       MemberType = "admin"
	ChatParticipant MemberType = "participant"
)

type Chat struct {
	ID        uuid.UUID
	Type      ChatType
	Title     *string
	Members   map[uuid.UUID]MemberType
	CreatedAt time.Time
}

func NewChat(chatType ChatType, title *string, members map[uuid.UUID]MemberType) (*Chat, error) {
	chat := &Chat{
		ID:        uuid.New(),
		Type:      chatType,
		Title:     title,
		Members:   members,
		CreatedAt: time.Now().UTC(),
	}

	if err := chat.Validate(); err != nil {
		return nil, err
	}

	return chat, nil
}

func (c *Chat) Validate() error {
	if c.ID == uuid.Nil {
		return errors.New("chat id cannot be empty")
	}

	if c.Type != ChatTypeDirect && c.Type != ChatTypeGroup {
		return fmt.Errorf("invalid chat type: %s", c.Type)
	}

	if len(c.Members) == 0 {
		return errors.New("chat must have at least one member")
	}

	if c.Type == ChatTypeDirect && len(c.Members) != 2 {
		return errors.New("direct chat must have exactly 2 members")
	}

	if c.Type == ChatTypeGroup && (c.Title == nil || strings.TrimSpace(*c.Title) == "") {
		return errors.New("group chat title cannot be empty")
	}

	return nil
}

func (c *Chat) IsMember(userID uuid.UUID) bool {
	_, exists := c.Members[userID]
	return exists
}

func (c *Chat) GetRole(userID uuid.UUID) (MemberType, bool) {
	role, exists := c.Members[userID]
	return role, exists
}

type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	SenderID  uuid.UUID
	Content   string
	CreatedAt time.Time
}

func NewMessage(chatID, senderID uuid.UUID, content string) (*Message, error) {
	msg := &Message{
		ID:        uuid.New(),
		ChatID:    chatID,
		SenderID:  senderID,
		Content:   strings.TrimSpace(content),
		CreatedAt: time.Now().UTC(),
	}

	if err := msg.Validate(); err != nil {
		return nil, err
	}

	return msg, nil
}

func (m *Message) Validate() error {
	if m.ID == uuid.Nil {
		return errors.New("message id cannot be empty")
	}
	if m.ChatID == uuid.Nil {
		return errors.New("chat_id cannot be empty")
	}
	if m.SenderID == uuid.Nil {
		return errors.New("sender_id cannot be empty")
	}

	if strings.TrimSpace(m.Content) == "" {
		return errors.New("message content cannot be empty")
	}

	if len(m.Content) > MaxMessageLength {
		return fmt.Errorf("message length exceeds max limit of %d characters", MaxMessageLength)
	}

	return nil
}
