package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/joho/godotenv"
	core_config "github.com/poponyas/GrpcChat/core/config"
	core_logger "github.com/poponyas/GrpcChat/core/logger"
	core_pgx_pool "github.com/poponyas/GrpcChat/core/repository/postgres/pgx"
	core_server_v1_grpc "github.com/poponyas/GrpcChat/core/server/grpc"
	adapters_client_v1 "github.com/poponyas/GrpcChat/internal/client/adapters"
	controllers_client_grpc_v1 "github.com/poponyas/GrpcChat/internal/client/controllers/grpc/v1"
	usecase_client_grpc_v1 "github.com/poponyas/GrpcChat/internal/client/usecase"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from OS environment")
	}
	cofg := core_config.NewConfigMust()
	time.Local = cofg.TimeZone

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	logger, err := core_logger.NewLogger(core_logger.NewConfigMust())
	if err != nil {
		fmt.Println("failed to init application logger:", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Debug("application time zone", zap.Any("zone", time.Local))
	logger.Debug("initializing postgres connection pool")

	pool, err := core_pgx_pool.NewPool(
		ctx,
		core_pgx_pool.NewConfigMust(),
	)
	if err != nil {
		logger.Fatal("failed to init postgres connection pool", zap.Error(err))
	}
	defer pool.Close()

	logger.Debug("initializing feature", zap.String("feature", "users"))
	repo := adapters_client_v1.NewUserRepository(pool)
	usecase := usecase_client_grpc_v1.NewUseCasegRPC(repo)
	handler := controllers_client_grpc_v1.NewTransportGRPC(usecase)

	// Инициализируем gRPC сервер через готовый компонент
	grpcServer, err := core_server_v1_grpc.New(core_server_v1_grpc.NewConfigMust(), handler, logger)
	if err != nil {
		logger.Fatal("failed to start gRPC server", zap.Error(err))
	}

	// Ожидаем сигнал SIGINT/SIGTERM для Graceful Shutdown
	<-ctx.Done()
	logger.Info("shutting down gRPC server...")

	grpcServer.Close()
	logger.Info("gRPC server stopped successfully")
}
