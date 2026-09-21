package usecase_client_grpc_v1

import (
	"context"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
)

type UseCasegRPC struct {
	repository ProfileRepository
}

type ProfileRepository interface {
	CreateProfile(ctx context.Context, input domain.UserProfile) (domain.UserProfile, error)
	GetProfile(ctx context.Context, id uuid.UUID) (domain.UserProfile, error)
	GetProfiles(ctx context.Context, limit int32, offset int32) ([]domain.UserProfile, error)
	DeleteProfile(ctx context.Context, id uuid.UUID) error
	UpdateProfile(ctx context.Context, input domain.UserProfile) (domain.UserProfile, error)
}

func NewUseCasegRPC(
	repo ProfileRepository,
) *UseCasegRPC {
	return &UseCasegRPC{
		repository: repo,
	}
}
