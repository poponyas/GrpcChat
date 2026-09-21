package core_grpc_interceptor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	core_logger "github.com/poponyas/GrpcChat/core/logger"
)

const RequestIDHeader = "x-request-id"

func UnaryServerInterceptor(log *core_logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := time.Now().UTC()

		md, ok := metadata.FromIncomingContext(ctx)
		var requestID string
		if ok && len(md.Get(RequestIDHeader)) > 0 {
			requestID = md.Get(RequestIDHeader)[0]
		} else {
			requestID = uuid.NewString()
		}

		reqLogger := log.With(
			zap.String("request_id", requestID),
			zap.String("grpc_method", info.FullMethod),
		)

		// Помещаем логгер с полем request_id в контекст для нижележащих слоев
		ctx = core_logger.ToContext(ctx, reqLogger)

		defer func() {
			if p := recover(); p != nil {
				reqLogger.Error("unexpected panic during gRPC request handling",
					zap.Any("panic", p),
					zap.Duration("latency", time.Since(start)),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		reqLogger.Debug(">>> incoming gRPC request")

		resp, err = handler(ctx, req)

		stCode := status.Code(err)
		if err != nil {
			reqLogger.Error("<<< done gRPC request with error",
				zap.String("status_code", stCode.String()),
				zap.Duration("latency", time.Since(start)),
				zap.Error(err),
			)
		} else {
			reqLogger.Info("<<< done gRPC request",
				zap.String("status_code", stCode.String()),
				zap.Duration("latency", time.Since(start)),
			)
		}

		return resp, err
	}
}
