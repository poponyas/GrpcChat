package controllers_client_grpc_v1

import (
	"context"
	"errors"

	"github.com/poponyas/GrpcChat/core/domain"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (t *TransportgRPC) UpdateProfile(ctx context.Context, in *chat_v1.UpdateProfileInput) (*chat_v1.UserProfile, error) {
	userUpdate := domain.UserUpdateRequest{
		ID:        in.GetId(),
		Name:      in.GetName(),
		FullName:  in.GetFullname(),
		Phone:     in.Phone,
		AvatarURL: in.AvatarUrl,
	}

	response, err := t.usecase.UpdateProfile(ctx, userUpdate)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.InvalidArgument, "invalid profile update data: %v", err)
	}

	return dto_grpc_v1.FromDTOToUserProfile(response), nil
}
