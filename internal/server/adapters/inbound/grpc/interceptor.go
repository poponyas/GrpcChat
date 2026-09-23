package adapters_inbound_grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	pkg_context "github.com/poponyas/GrpcChat/pkg/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// wrappedStream переопределяет метод Context() для передачи обновленного контекста в хэндлер
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// UnaryAuthInterceptor обрабатывает обычные gRPC вызовы
func UnaryAuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		userID, err := authenticateContext(ctx)
		if err != nil {
			return nil, err
		}

		newCtx := context.WithValue(ctx, pkg_context.UserIDKey, userID)
		return handler(newCtx, req)
	}
}

// StreamAuthInterceptor обрабатывает стриминг (например, Connect)
func StreamAuthInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		userID, err := authenticateContext(ss.Context())
		if err != nil {
			return err
		}

		newCtx := context.WithValue(ss.Context(), pkg_context.UserIDKey, userID)

		// Оборачиваем stream, подменяя контекст
		wrapped := &wrappedStream{
			ServerStream: ss,
			ctx:          newCtx,
		}

		return handler(srv, wrapped)
	}
}

// authenticateContext извлекает Authorization Bearer token из метаданных
func authenticateContext(ctx context.Context) (uuid.UUID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.Nil, status.Error(codes.Unauthenticated, "metadata is missing")
	}

	// Вариант 2: Заголовок authorization (Bearer token)
	authHeaders := md.Get("authorization")
	if len(authHeaders) > 0 {
		rawToken := authHeaders[0]
		tokenStr := strings.TrimPrefix(rawToken, "Bearer ")

		// TODO: Здесь вызываем валидацию JWT токена (например, jwt.Parse)
		// и достаем user_id из claims
		_ = tokenStr
	}

	return uuid.Nil, status.Error(codes.Unauthenticated, "authentication token or x-user-id is required")
}
