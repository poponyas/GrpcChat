package usecase_client_grpc_v1

import (
	"context"
	"fmt"

	"github.com/poponyas/GrpcChat/core/domain"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
)

func (u *UseCasegRPC) CreateProfile(ctx context.Context, input dto_grpc_v1.CreateProfileRequest) (dto_grpc_v1.ProfileResponse, error) {
	userUninitialized := domain.NewUserProfile(
		input.Name,
		input.FullName,
		input.Phone,
		input.AvatarURL,
	)

	if err := userUninitialized.Validate(); err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("validate error: %w", err)
	}

	user, err := u.repository.CreateProfile(ctx, userUninitialized)
	if err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("can't create profile in usecase: %w", err)
	}

	// Возвращаем DTO через маппер из domain.UserProfile
	return dto_grpc_v1.FromDomainToProfileResponse(user), nil
}
