package benchsvc

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgStore is a tenant-agnostic PostgreSQL repository for benchmark data.
// All query methods accept tenantID as a parameter rather than capturing
// it at construction time.
type PgStore struct {
	db *pgxpool.Pool
}

// NewPgStore creates a new PgStore backed by the given connection pool.
func NewPgStore(db *pgxpool.Pool) *PgStore {
	return &PgStore{db: db}
}

// BeginTx starts a new database transaction.
func (s *PgStore) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.db.Begin(ctx)
}
