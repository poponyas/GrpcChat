package adapters_client_v1

import core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"

type UserRepository struct {
	Pool core_postgres_pool.Pool
}

func NewUserRepository(
	pool core_postgres_pool.Pool,
) *UserRepository {
	return &UserRepository{
		Pool: pool,
	}
}
