package pkg_context

import (
	"context"
	"github.com/google/uuid"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return val, ok && val != uuid.Nil
}
