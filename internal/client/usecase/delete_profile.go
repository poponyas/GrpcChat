package usecase_client_grpc_v1

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (u *UseCasegRPC) DeleteProfile(ctx context.Context, id string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("can't parse id: %w", err)
	}

	err = u.repository.DeleteProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("can't delete user: UseCase: %w", err)
	}

	return nil
}
