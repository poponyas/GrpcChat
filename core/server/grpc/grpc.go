package core_server_v1_grpc

import (
	"fmt"
	"net"

	core_grpc_interceptor "github.com/poponyas/GrpcChat/core/interceptors"
	core_logger "github.com/poponyas/GrpcChat/core/logger"
	chat_v1 "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	server *grpc.Server
}

func New(
	c Config,
	userService chat_v1.UserServiceServer,
	logger *core_logger.Logger,
) (*Server, error) {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(core_grpc_interceptor.UnaryServerInterceptor(logger)),
	)

	// Регистрируем хэндлер и рефлексию
	chat_v1.RegisterUserServiceServer(s, userService)
	reflection.Register(s)

	if err := start(s, c.Port, logger); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	return &Server{server: s}, nil
}

func start(server *grpc.Server, port string, logger *core_logger.Logger) error {
	conn, err := net.Listen("tcp", net.JoinHostPort("", port))
	if err != nil {
		return fmt.Errorf("net.Listen: %w", err)
	}

	go func() {
		if err := server.Serve(conn); err != nil && err != grpc.ErrServerStopped {
			logger.Fatal("can't serve gRPC", zap.Error(err))
		}
	}()

	logger.Info("gRPC server started", zap.String("port", port))

	return nil
}

func (s *Server) Close() {
	s.server.GracefulStop()
}
