package repository

import (
	"fmt"
	"time"

	"github.com/GoPolymarket/polygate/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/jmoiron/sqlx"
)

func NewDB(cfg *config.Config) (*sqlx.DB, error) {
	dsn := "postgres://postgres:postgres@localhost:5432/polygate?sslmode=disable"
	if cfg != nil && cfg.Database.DSN != "" {
		dsn = cfg.Database.DSN
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	// 连接池设置
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(1 * time.Hour)

	return db, nil
}
