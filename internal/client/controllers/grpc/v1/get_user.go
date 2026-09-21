package controllers_client_grpc_v1

import (
	"context"
	"errors"

	"github.com/google/uuid"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProfileResponse dto_grpc_v1.ProfileResponse

func (t *TransportgRPC) GetProfile(ctx context.Context, in *chat_v1.GetProfileInput) (*chat_v1.UserProfile, error) {
	id, err := uuid.Parse(in.Id)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id: %v", err)
	}

	response, err := t.usecase.GetProfile(ctx, id)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return dto_grpc_v1.FromDTOToUserProfile(response), nil
}
