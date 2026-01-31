package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type PostgresRiskRepo struct {
	db *sqlx.DB
}

func NewPostgresRiskRepo(db *sqlx.DB) *PostgresRiskRepo {
	repo := &PostgresRiskRepo{db: db}
	_ = repo.ensureSchema(context.Background())
	return repo
}

// GetDailyUsage 获取当日已用额度与订单数
func (r *PostgresRiskRepo) GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var orders int
	var vol float64
	query := `SELECT orders, volume FROM risk_daily_usage WHERE tenant_id = $1 AND date = $2`

	err := r.db.QueryRowxContext(ctx, query, tenantID, today).Scan(&orders, &vol)
	if err != nil {
		// 如果没找到，就是 0
		return 0, 0, nil
	}
	return orders, vol, nil
}

// AddDailyUsage 原子增加额度与订单数
func (r *PostgresRiskRepo) AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error {
	today := time.Now().UTC().Format("2006-01-02")

	// Upsert (Insert or Update)
	query := `
		INSERT INTO risk_daily_usage (tenant_id, date, orders, volume)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, date)
		DO UPDATE SET orders = risk_daily_usage.orders + $3,
		              volume = risk_daily_usage.volume + $4
	`

	_, err := r.db.ExecContext(ctx, query, tenantID, today, orders, amount)
	return err
}

func (r *PostgresRiskRepo) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS risk_daily_usage (
			tenant_id TEXT NOT NULL,
			date DATE NOT NULL,
			orders INTEGER NOT NULL DEFAULT 0,
			volume DOUBLE PRECISION NOT NULL DEFAULT 0,
			PRIMARY KEY (tenant_id, date)
		)
	`)
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, `ALTER TABLE risk_daily_usage ADD COLUMN IF NOT EXISTS orders INTEGER NOT NULL DEFAULT 0`)
	_, _ = r.db.ExecContext(ctx, `ALTER TABLE risk_daily_usage ADD COLUMN IF NOT EXISTS volume DOUBLE PRECISION NOT NULL DEFAULT 0`)
	return nil
}

func (r *PostgresRiskRepo) Cleanup(ctx context.Context, olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	_, err := r.db.ExecContext(ctx, `DELETE FROM risk_daily_usage WHERE date < $1`, cutoff.Format("2006-01-02"))
	return err
}
