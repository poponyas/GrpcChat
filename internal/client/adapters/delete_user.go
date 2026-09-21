package adapters_client_v1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
)

func (r *UserRepository) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.Pool.OperationTimeout())
	defer cancel()

	query := `
	DELETE FROM chat.users
	WHERE id = $1;
	`

	cmdTg, err := r.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	if cmdTg.RowsAffected() == 0 {
		return core_postgres_pool.ErrNoRows
	}

	return nil
}
