package core_grpc_interceptor

import (
	"context"
	"time"

	core_logger "github.com/poponyas/GrpcChat/core/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(logger *core_logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		st, _ := status.FromError(err)

		fields := []zap.Field{
			zap.String("method", info.FullMethod),
			zap.Duration("duration", duration),
			zap.String("code", st.Code().String()),
		}

		if err != nil {
			logger.Error("gRPC call failed", append(fields, zap.Error(err))...)
		} else {
			logger.Info("gRPC call success", fields...)
		}

		return resp, err
	}
}
