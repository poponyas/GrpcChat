package pkg_context

import "context"

type contextKey string

const UserIDKey contextKey = "user_id"

func GetUserID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(UserIDKey).(string)
	return val, ok
}
