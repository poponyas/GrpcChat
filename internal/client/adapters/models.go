package adapters_client_v1

import (
	"time"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
)

type UserModel struct {
	ID            uuid.UUID
	Name          string
	Fullname      string
	Phone         *string
	AvatarURL     *string
	PhotoPostedAt *time.Time
	CreatedAt     time.Time
}

func modelToDomain(model UserModel) domain.UserProfile {
	return domain.UserProfile{
		ID:            model.ID,
		Name:          model.Name,
		Fullname:      model.Fullname,
		Phone:         model.Phone,
		AvatarURL:     model.AvatarURL,
		PhotoPostedAt: model.PhotoPostedAt,
		CreatedAt:     model.CreatedAt,
	}
}
