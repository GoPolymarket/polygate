package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	Client *gorm.DB
}

func NewDB(cfg *config.Config) (*DB, error) {
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("database dsn is empty")
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &DB{Client: db}, nil
}

type PostgresAuditRepo struct {
	db *DB
}

func NewPostgresAuditRepo(db *DB) *PostgresAuditRepo {
	return &PostgresAuditRepo{db: db}
}

func (r *PostgresAuditRepo) Insert(ctx context.Context, entry *model.AuditLog) error {
	return r.db.Client.WithContext(ctx).Create(entry).Error
}

func (r *PostgresAuditRepo) List(ctx context.Context, tenantID string, limit int, from, to *time.Time) ([]*model.AuditLog, error) {
	var logs []*model.AuditLog
	tx := r.db.Client.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if from != nil {
		tx = tx.Where("created_at >= ?", from)
	}
	if to != nil {
		tx = tx.Where("created_at <= ?", to)
	}

	err := tx.Order("created_at desc").Limit(limit).Find(&logs).Error
	return logs, err
}
