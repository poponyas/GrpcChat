package controllers_client_grpc_v1

import (
	"context"

	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	dto_grpc_v1 "github.com/poponyas/GrpcChat/internal/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (t *TransportgRPC) CreateProfile(ctx context.Context, in *chat_v1.CreateProfileInput) (*chat_v1.UserProfile, error) {
	input := dto_grpc_v1.CreateProfileRequest{
		Name:      in.GetName(),
		FullName:  in.GetFullname(),
		Phone:     in.Phone,
		AvatarURL: in.AvatarUrl,
	}

	output, err := t.usecase.CreateProfile(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't create profile: CreateProfile: %v", err)
	}

	response := dto_grpc_v1.FromDTOToUserProfile(output)

	return response, nil
}
