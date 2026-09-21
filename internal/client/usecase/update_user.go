package usecase_client_grpc_v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
)

func (u *UseCasegRPC) UpdateProfile(ctx context.Context, in domain.UserUpdateRequest) (dto_grpc_v1.ProfileResponse, error) {
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("failed to parse uuid from req: %w", err)
	}
	user, err := u.repository.GetProfile(ctx, id)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return dto_grpc_v1.ProfileResponse{}, core_postgres_pool.ErrNoRows
		}
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("can't get user from usecase: %w", err)
	}

	if err := user.ApplyPatch(in); err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("failed to apply and validate userPatch: %w", err)
	}

	response, err := u.repository.UpdateProfile(ctx, user)
	if err != nil {
		return dto_grpc_v1.ProfileResponse{}, fmt.Errorf("can't update user: %w", err)
	}

	return dto_grpc_v1.FromDomainToProfileResponse(response), nil
}
