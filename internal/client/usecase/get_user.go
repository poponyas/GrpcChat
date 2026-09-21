package usecase_client_grpc_v1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
)

func (u *UseCasegRPC) GetProfile(ctx context.Context, id uuid.UUID) (dto_grpc_v1.ProfileResponse, error) {
	user, err := u.repository.GetProfile(ctx, id)
	if err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("can't get user from repo: GetProfile: %w", err)
	}

	return dto_grpc_v1.FromDomainToProfileResponse(user), nil
}
