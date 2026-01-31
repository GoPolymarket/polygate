package repository

import (
	"context"
	"time"

	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/jmoiron/sqlx"
)

type PostgresIdempotencyStore struct {
	db *sqlx.DB
}

func NewPostgresIdempotencyStore(db *sqlx.DB) *PostgresIdempotencyStore {
	store := &PostgresIdempotencyStore{db: db}
	_ = store.ensureSchema(context.Background())
	return store
}

func (s *PostgresIdempotencyStore) GetOrLock(key string) (*middleware.IdempotencyRecord, bool) {
	ctx := context.Background()
	now := time.Now().UTC()
	result, _ := s.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (key, processing, created_at)
		VALUES ($1, true, $2)
		ON CONFLICT (key) DO NOTHING
	`, key, now)
	if rows, _ := result.RowsAffected(); rows > 0 {
		return nil, false
	}

	var rec middleware.IdempotencyRecord
	err := s.db.QueryRowxContext(ctx, `
		SELECT status_code, response_body, created_at, processing
		FROM idempotency_keys
		WHERE key = $1
	`, key).Scan(&rec.Status, &rec.Body, &rec.CreatedAt, &rec.Processing)
	if err != nil {
		return nil, false
	}
	return &rec, true
}

func (s *PostgresIdempotencyStore) Save(key string, status int, body []byte) {
	ctx := context.Background()
	_, _ = s.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status_code = $2, response_body = $3, processing = false
		WHERE key = $1
	`, key, status, body)
}

func (s *PostgresIdempotencyStore) Unlock(key string) {
	ctx := context.Background()
	_, _ = s.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE key = $1`, key)
}

func (s *PostgresIdempotencyStore) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS idempotency_keys (
			key TEXT PRIMARY KEY,
			status_code INTEGER NOT NULL DEFAULT 0,
			response_body BYTEA,
			processing BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS status_code INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS response_body BYTEA`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS processing BOOLEAN NOT NULL DEFAULT true`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE idempotency_keys ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now()`)
	return nil
}

func (s *PostgresIdempotencyStore) Cleanup(ctx context.Context, olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	_, err := s.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE created_at < $1`, cutoff)
	return err
}
