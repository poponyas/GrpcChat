package core_server_v1_grpc

import (
	"fmt"
	"net"
	"time"

	core_grpc_interceptor "github.com/poponyas/GrpcChat/core/interceptors"
	core_logger "github.com/poponyas/GrpcChat/core/logger"
	chat_v1 "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"github.com/poponyas/GrpcChat/internal/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	server *grpc.Server
}

func New(
	c Config,
	userService chat_v1.UserServiceServer,
	chatService chat_v1.ChatServiceServer,
	authService *auth.Service,
	logger *core_logger.Logger,
) (*Server, error) {
	options := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(core_grpc_interceptor.UnaryServerInterceptor(logger), authService.UnaryInterceptor()),
		grpc.ChainStreamInterceptor(authService.StreamInterceptor()),
		grpc.MaxRecvMsgSize(64 * 1024),
	}
	if c.TLSCert != "" || c.TLSKey != "" {
		creds, err := credentials.NewServerTLSFromFile(c.TLSCert, c.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load TLS certificate: %w", err)
		}
		options = append(options, grpc.Creds(creds))
	} else if ip := net.ParseIP(c.Host); c.Host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("non-loopback GRPC_HOST requires GRPC_TLS_CERT and GRPC_TLS_KEY")
	}
	s := grpc.NewServer(options...)

	// Регистрируем хэндлер и рефлексию
	chat_v1.RegisterUserServiceServer(s, userService)
	chat_v1.RegisterChatServiceServer(s, chatService)
	chat_v1.RegisterAuthServiceServer(s, authService)
	reflection.Register(s)

	if err := start(s, net.JoinHostPort(c.Host, c.Port), logger); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	return &Server{server: s}, nil
}

func start(server *grpc.Server, address string, logger *core_logger.Logger) error {
	conn, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("net.Listen: %w", err)
	}

	go func() {
		if err := server.Serve(conn); err != nil && err != grpc.ErrServerStopped {
			logger.Fatal("can't serve gRPC", zap.Error(err))
		}
	}()

	logger.Info("gRPC server started", zap.String("address", address))

	return nil
}

func (s *Server) Close() {
	done := make(chan struct{})
	go func() { s.server.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.server.Stop()
		<-done
	}
}
