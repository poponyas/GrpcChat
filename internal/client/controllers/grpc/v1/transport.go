package controllers_client_grpc_v1

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
)

type TransportgRPC struct {
	chat_v1.UnimplementedUserServiceServer

	usecase ProfileUseCase
}

type ProfileUseCase interface {
	CreateProfile(ctx context.Context, input dto_grpc_v1.CreateProfileRequest) (dto_grpc_v1.ProfileResponse, error)
	GetProfile(ctx context.Context, id uuid.UUID) (dto_grpc_v1.ProfileResponse, error)
	GetProfiles(ctx context.Context, limit int32, offset int32) ([]dto_grpc_v1.ProfileResponse, error)
	DeleteProfile(ctx context.Context, id string) error
	UpdateProfile(ctx context.Context, in domain.UserUpdateRequest) (dto_grpc_v1.ProfileResponse, error)
}

func NewTransportGRPC(
	usecase ProfileUseCase,
) *TransportgRPC {
	return &TransportgRPC{
		usecase: usecase,
	}
}
