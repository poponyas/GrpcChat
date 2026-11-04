package controllers_client_grpc_v1

import (
	"context"
	"errors"

	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetProfilesList []chat_v1.UserProfile

func (t *TransportgRPC) GetProfiles(ctx context.Context, in *chat_v1.GetProfilesInput) (*chat_v1.GetProfilesOutput, error) {
	limit, offset := in.GetLimit(), in.GetOffset()
	if limit < 0 || offset < 0 {
		return nil, status.Error(codes.InvalidArgument, "limit and offset cannot be negative")
	}

	response, err := t.usecase.GetProfiles(ctx, limit, offset)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &chat_v1.GetProfilesOutput{
		Profiles: dto_grpc_v1.FromDTOsToUserProfiles(response),
	}, nil
}
