package controllers_client_grpc_v1

import (
	"context"
	"errors"
	"strings"

	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (t *TransportgRPC) DeleteProfile(ctx context.Context, in *chat_v1.DeleteProfileInput) (*emptypb.Empty, error) {
	id := in.GetId()
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	err := t.usecase.DeleteProfile(ctx, id)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user with id %s not found", id)
		}

		if strings.Contains(err.Error(), "invalid uuid format") {
			return nil, status.Errorf(codes.InvalidArgument, "invalid uuid string: %s", id)
		}

		return nil, status.Errorf(codes.Internal, "internal error during user deletion")
	}

	return &emptypb.Empty{}, nil
}
