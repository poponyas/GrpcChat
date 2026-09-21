package adapters_client_v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/poponyas/GrpcChat/core/domain"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
)

func (r *UserRepository) GetProfile(ctx context.Context, id uuid.UUID) (domain.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OperationTimeout())
	defer cancel()

	query := `
    SELECT id, name, fullname, phone, avatar_url, photo_posted_at, created_at
    FROM chat.users
    WHERE id=$1;
    `

	row := r.Pool.QueryRow(ctx, query, id)

	var userModel UserModel
	err := row.Scan(
		&userModel.ID,
		&userModel.Name,
		&userModel.Fullname,
		&userModel.Phone,
		&userModel.AvatarURL,
		&userModel.PhotoPostedAt,
		&userModel.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return domain.UserProfile{}, fmt.Errorf("user not found: %w", core_postgres_pool.ErrNoRows)
		}
		return domain.UserProfile{}, fmt.Errorf("failed to scan userModel: %w", err)
	}

	return modelToDomain(userModel), nil
}
