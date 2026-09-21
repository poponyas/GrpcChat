package usecase_client_grpc_v1

import (
	"context"
	"fmt"

	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
)

const (
	defaultLimit = 10
	maxLimit     = 100
)

func (u *UseCasegRPC) GetProfiles(
	ctx context.Context, limit int32, offset int32,
) ([]dto_grpc_v1.ProfileResponse, error) {
	if limit == 0 {
		limit = defaultLimit
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	users, err := u.repository.GetProfiles(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("can't get profiles from repo: %w", err)
	}

	var result []dto_grpc_v1.ProfileResponse
	for _, user := range users {
		result = append(result, dto_grpc_v1.FromDomainToProfileResponse(user))
	}

	return result, nil
}
