package adapters_client_v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/poponyas/GrpcChat/core/domain"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
)

func (r *UserRepository) UpdateProfile(ctx context.Context, input domain.UserProfile) (domain.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OperationTimeout())
	defer cancel()

	query := `
    UPDATE chat.users
    SET 
        name = $1,
        fullname = $2,
        phone = $3,
        avatar_url = $4,
        photo_posted_at = $5
    WHERE id = $6
    RETURNING id, name, fullname, phone, avatar_url, photo_posted_at, created_at;
    `

	row := r.Pool.QueryRow(
		ctx,
		query,
		input.Name,
		input.Fullname,
		input.Phone,
		input.AvatarURL,
		input.PhotoPostedAt,
		input.ID,
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
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return domain.UserProfile{}, fmt.Errorf("user not found: %w", core_postgres_pool.ErrNoRows)
		}
		return domain.UserProfile{}, fmt.Errorf("failed to scan userModel: %w", err)
	}

	return modelToDomain(userModel), nil
}
