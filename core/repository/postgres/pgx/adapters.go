package core_pgx_pool

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	core_postgres_pool "github.com/poponyas/GrpcChat/core/repository/postgres"
)

type pgxRows struct {
	pgx.Rows
}

type pgxRow struct {
	row pgx.Row
}

func (r pgxRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if err != nil {
		return mapErrors(err)
	}
	return nil
}

type pgxCommandTag struct {
	tag pgconn.CommandTag
}

func (c pgxCommandTag) RowsAffected() int64 {
	return c.tag.RowsAffected()
}

func (r pgxRows) Scan(dest ...any) error {
	if err := r.Rows.Scan(dest...); err != nil {
		return mapErrors(err)
	}
	return nil
}

func (r pgxRows) Err() error {
	if err := r.Rows.Err(); err != nil {
		return mapErrors(err)
	}
	return nil
}

func mapErrors(err error) error {
	const (
		pgxViolatesForeignKeyErrorCode = "23503"
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return core_postgres_pool.ErrNoRows
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		if pgErr.Code == pgxViolatesForeignKeyErrorCode {
			return fmt.Errorf(
				"%v: %w",
				err,
				core_postgres_pool.ErrViolatesForeignKey,
			)
		}
	}

	return fmt.Errorf(
		"%v: %w",
		err,
		core_postgres_pool.ErrUnknown,
	)
}
