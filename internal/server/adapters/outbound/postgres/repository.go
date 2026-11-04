package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/poponyas/GrpcChat/core/domain"
	service "github.com/poponyas/GrpcChat/internal/server/service"
)

// Repository keeps membership changes and message authorization in transactions.
type Repository struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{Pool: pool} }
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrChatNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23503" {
		return service.ErrUserNotFound
	}
	return err
}

func loadChat(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Chat, error) {
	c := &domain.Chat{Members: make(map[uuid.UUID]domain.MemberType), MemberNames: make(map[uuid.UUID]string)}
	var name string
	var direct bool
	err := tx.QueryRow(ctx, `SELECT id,name,is_private,created_at FROM chat.chats WHERE id=$1 FOR UPDATE`, id).Scan(&c.ID, &name, &direct, &c.CreatedAt)
	if err != nil {
		return nil, dbError(err)
	}
	c.Type = domain.ChatTypeGroup
	c.Title = &name
	if direct {
		c.Type = domain.ChatTypeDirect
	}
	rows, err := tx.Query(ctx, `SELECT m.user_id,m.role,u.name FROM chat.chat_members m JOIN chat.users u ON u.id=m.user_id WHERE chat_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var role domain.MemberType
		var name string
		if err := rows.Scan(&id, &role, &name); err != nil {
			return nil, err
		}
		c.Members[id] = role
		c.MemberNames[id] = name
	}
	return c, rows.Err()
}
func (r *Repository) GetChatByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	c, err := loadChat(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return c, tx.Commit(ctx)
}
func (r *Repository) SaveChat(ctx context.Context, c *domain.Chat) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	ids := make([]string, 0, len(c.Members))
	for id := range c.Members {
		ids = append(ids, id.String())
	}
	sort.Strings(ids)
	var key *string
	if c.Type == domain.ChatTypeDirect {
		k := strings.Join(ids, ":")
		key = &k
	}
	title := ""
	if c.Title != nil {
		title = *c.Title
	}
	// ON CONFLICT makes simultaneous /dm requests converge on one conversation.
	err = tx.QueryRow(ctx, `INSERT INTO chat.chats(id,name,is_private,created_at,direct_key) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(direct_key) DO UPDATE SET direct_key=EXCLUDED.direct_key RETURNING id,created_at`, c.ID, title, c.Type == domain.ChatTypeDirect, c.CreatedAt, key).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return dbError(err)
	}
	for _, raw := range ids {
		id := uuid.MustParse(raw)
		_, err = tx.Exec(ctx, `INSERT INTO chat.chat_members(chat_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, c.ID, id, c.Members[id])
		if err != nil {
			return dbError(err)
		}
	}
	return tx.Commit(ctx)
}
func (r *Repository) ListChats(ctx context.Context, user uuid.UUID, limit, offset int) ([]*domain.Chat, error) {
	rows, err := r.Pool.Query(ctx, `SELECT c.id FROM chat.chats c JOIN chat.chat_members m ON m.chat_id=c.id WHERE m.user_id=$1 ORDER BY c.created_at DESC,c.id DESC LIMIT $2 OFFSET $3`, user, limit, offset)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var chats []*domain.Chat
	for _, id := range ids {
		c, err := r.GetChatByID(ctx, id)
		if errors.Is(err, service.ErrChatNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if c.IsMember(user) {
			chats = append(chats, c)
		}
	}
	return chats, nil
}
func (r *Repository) ChangeMember(ctx context.Context, actor, chat, user uuid.UUID, remove bool) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := loadChat(ctx, tx, chat)
	if err != nil {
		return err
	}
	if !c.IsMember(actor) {
		return service.ErrAccessDenied
	}
	if c.Type == domain.ChatTypeDirect {
		return fmt.Errorf("%w: direct chat membership is fixed", service.ErrInvalidInput)
	}
	role := c.Members[actor]
	if remove && actor == user {
		if role == domain.ChatOwner && len(c.Members) > 1 {
			return fmt.Errorf("%w: owner must remove other members before leaving", service.ErrInvalidInput)
		}
	} else if role != domain.ChatOwner && role != domain.ChatAdmin {
		return service.ErrAccessDenied
	}
	if remove {
		if c.Members[user] == domain.ChatOwner && !(actor == user && len(c.Members) == 1) {
			return fmt.Errorf("%w: cannot remove group owner", service.ErrInvalidInput)
		}
		// A sole owner can leave, deleting the now empty group.
		if actor == user && role == domain.ChatOwner && len(c.Members) == 1 {
			_, err = tx.Exec(ctx, `DELETE FROM chat.chats WHERE id=$1`, chat)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM chat.chat_members WHERE chat_id=$1 AND user_id=$2`, chat, user)
		}
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO chat.chat_members(chat_id,user_id,role) VALUES($1,$2,'participant') ON CONFLICT DO NOTHING`, chat, user)
	}
	if err != nil {
		return dbError(err)
	}
	return tx.Commit(ctx)
}
func (r *Repository) SaveMessage(ctx context.Context, msg *domain.Message) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := loadChat(ctx, tx, msg.ChatID)
	if err != nil {
		return err
	}
	if !c.IsMember(msg.SenderID) {
		return service.ErrAccessDenied
	}
	err = tx.QueryRow(ctx, `INSERT INTO chat.messages(id,chat_id,sender_id,content,created_at) VALUES($1,$2,$3,$4,$5) RETURNING (SELECT name FROM chat.users WHERE id=$3)`, msg.ID, msg.ChatID, msg.SenderID, msg.Content, msg.CreatedAt).Scan(&msg.SenderName)
	if err != nil {
		return dbError(err)
	}
	return tx.Commit(ctx)
}
func (r *Repository) GetHistory(ctx context.Context, chat, user uuid.UUID, limit int, before *uuid.UUID) ([]*domain.Message, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	c, err := loadChat(ctx, tx, chat)
	if err != nil {
		return nil, err
	}
	if !c.IsMember(user) {
		return nil, service.ErrAccessDenied
	}
	// A cursor must belong to this conversation; foreign cursors never expose history.
	var beforeTime *time.Time
	if before != nil {
		var t time.Time
		err := tx.QueryRow(ctx, `SELECT created_at FROM chat.messages WHERE id=$1 AND chat_id=$2`, *before, chat).Scan(&t)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: history cursor not found in this chat", service.ErrInvalidInput)
		}
		if err != nil {
			return nil, err
		}
		beforeTime = &t
	}
	rows, err := tx.Query(ctx, `SELECT m.id,m.chat_id,coalesce(m.sender_id,'00000000-0000-0000-0000-000000000000'::uuid),coalesce(u.name,'deleted'),m.content,m.created_at
 FROM chat.messages m LEFT JOIN chat.users u ON u.id=m.sender_id WHERE m.chat_id=$1 AND ($2::timestamptz IS NULL OR (m.created_at,m.id)<($2,$3::uuid)) ORDER BY m.created_at DESC,m.id DESC LIMIT $4`, chat, beforeTime, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*domain.Message
	for rows.Next() {
		m := new(domain.Message)
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.SenderName, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return msgs, tx.Commit(ctx)
}
