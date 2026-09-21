package dto_grpc_v1

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	"github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CreateProfileRequest struct {
	Name      string
	FullName  string
	Phone     *string
	AvatarURL *string
}

type ProfileResponse struct {
	ID            uuid.UUID
	Name          string
	Fullname      string
	Phone         *string
	AvatarURL     *string
	PhotoPostedAt *time.Time
	CreatedAt     time.Time
}

func FromDTOToUserProfile(profile ProfileResponse) *chat_v1.UserProfile {
	// Безопасная конвертация опционального времени в Protobuf Timestamp
	var photoPostedAt *timestamppb.Timestamp
	if profile.PhotoPostedAt != nil {
		photoPostedAt = timestamppb.New(*profile.PhotoPostedAt)
	}

	return &chat_v1.UserProfile{
		Id:            profile.ID.String(),
		Name:          profile.Name,
		Fullname:      profile.Fullname,
		Phone:         profile.Phone,
		AvatarUrl:     profile.AvatarURL,
		PhotoPostedAt: photoPostedAt,
		CreatedAt:     timestamppb.New(profile.CreatedAt),
	}
}

func FromDTOsToUserProfiles(profiles []ProfileResponse) []*chat_v1.UserProfile {
	userProfiles := make([]*chat_v1.UserProfile, len(profiles))
	for i, profile := range profiles {
		userProfiles[i] = FromDTOToUserProfile(profile)
	}

	return userProfiles
}

func FromDomainToProfileResponse(user domain.UserProfile) ProfileResponse {
	return ProfileResponse{
		ID:            user.ID,
		Name:          user.Name,
		Fullname:      user.Fullname,
		Phone:         user.Phone,
		AvatarURL:     user.AvatarURL,
		PhotoPostedAt: user.PhotoPostedAt,
		CreatedAt:     user.CreatedAt,
	}
}
