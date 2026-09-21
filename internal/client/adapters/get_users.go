package adapters_client_v1

import (
	"context"
	"fmt"

	"github.com/poponyas/GrpcChat/core/domain"
)

func (r *UserRepository) GetProfiles(ctx context.Context, limit, offset int32) ([]domain.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OperationTimeout())
	defer cancel()

	query := `
    SELECT id, name, fullname, phone, avatar_url, photo_posted_at, created_at
    FROM chat.users
    ORDER BY created_at DESC -- Обязательно для предсказуемой пагинации!
    LIMIT $1 OFFSET $2;
    `

	rows, err := r.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query profiles: %w", err)
	}
	defer rows.Close()

	var profiles []domain.UserProfile
	for rows.Next() {
		var userModel UserModel
		err := rows.Scan(
			&userModel.ID,
			&userModel.Name,
			&userModel.Fullname,
			&userModel.Phone,
			&userModel.AvatarURL,
			&userModel.PhotoPostedAt,
			&userModel.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan userModel: %w", err)
		}

		profiles = append(profiles, modelToDomain(userModel))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return profiles, nil
}
