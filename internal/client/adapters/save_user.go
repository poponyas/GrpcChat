package adapters_client_v1

import (
	"context"
	"fmt"

	"github.com/poponyas/GrpcChat/core/domain"
)

func (r *UserRepository) CreateProfile(ctx context.Context, input domain.UserProfile) (domain.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OperationTimeout())
	defer cancel()

	query := `
	INSERT INTO chat.users (
    id, name, fullname, phone, avatar_url, photo_posted_at, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id, name, fullname, phone, avatar_url, photo_posted_at, created_at;
	`

	row := r.Pool.QueryRow(ctx,
		query,
		input.ID,
		input.Name,
		input.Fullname,
		input.Phone,
		input.AvatarURL,
		input.PhotoPostedAt,
		input.CreatedAt,
	)

	var userModel UserModel

	if err := row.Scan(
		&userModel.ID,
		&userModel.Name,
		&userModel.Fullname,
		&userModel.Phone,
		&userModel.AvatarURL,
		&userModel.PhotoPostedAt,
		&userModel.CreatedAt,
	); err != nil {
		return domain.UserProfile{},
			fmt.Errorf("can't get user model: %w", err)
	}

	return modelToDomain(userModel), nil

}
