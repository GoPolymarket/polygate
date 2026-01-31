package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/jmoiron/sqlx"
)

type PostgresTenantRepo struct {
	db *sqlx.DB
}

func NewPostgresTenantRepo(db *sqlx.DB) *PostgresTenantRepo {
	repo := &PostgresTenantRepo{db: db}
	_ = repo.ensureSchema(context.Background())
	return repo
}

// DB Model 用于处理 JSONB 序列化
type tenantDB struct {
	ID             string `db:"id"`
	Name           string `db:"name"`
	ApiKey         string `db:"api_key"`
	CredsJSON      []byte `db:"creds"`
	RiskConfigJSON []byte `db:"risk_config"`
	RateLimitJSON  []byte `db:"rate_limit_config"`
	SignersJSON    []byte `db:"allowed_signers"`
	CreatedAt      string `db:"created_at"` // 简化处理
}

func (r *PostgresTenantRepo) GetByApiKey(ctx context.Context, apiKey string) (*model.Tenant, error) {
	var td tenantDB
	query := `SELECT id, name, api_key, creds, risk_config, rate_limit_config, allowed_signers FROM tenants WHERE api_key = $1 LIMIT 1`

	err := r.db.GetContext(ctx, &td, query, apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}

	return r.toDomain(&td)
}

func (r *PostgresTenantRepo) toDomain(td *tenantDB) (*model.Tenant, error) {
	t := &model.Tenant{
		ID:     td.ID,
		Name:   td.Name,
		ApiKey: td.ApiKey,
	}

	if err := json.Unmarshal(td.CredsJSON, &t.Creds); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(td.RiskConfigJSON, &t.Risk); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(td.RateLimitJSON, &t.Rate); err != nil {
		return nil, err
	}
	if len(td.SignersJSON) > 0 {
		if err := json.Unmarshal(td.SignersJSON, &t.AllowedSigners); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// Create 用于初始化数据
func (r *PostgresTenantRepo) Create(ctx context.Context, t *model.Tenant) error {
	creds, _ := json.Marshal(t.Creds)
	risk, _ := json.Marshal(t.Risk)
	rate, _ := json.Marshal(t.Rate)
	signers, _ := json.Marshal(t.AllowedSigners)

	query := `INSERT INTO tenants (id, name, api_key, creds, risk_config, rate_limit_config, allowed_signers, created_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.ExecContext(ctx, query, t.ID, t.Name, t.ApiKey, creds, risk, rate, signers, time.Now().UTC())
	return err
}

func (r *PostgresTenantRepo) List(ctx context.Context, limit, offset int) ([]*model.Tenant, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, name, api_key, creds, risk_config, rate_limit_config, allowed_signers FROM tenants ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryxContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]*model.Tenant, 0, limit)
	for rows.Next() {
		var td tenantDB
		if err := rows.StructScan(&td); err != nil {
			return nil, err
		}
		tenant, err := r.toDomain(&td)
		if err != nil {
			return nil, err
		}
		results = append(results, tenant)
	}
	return results, nil
}

func (r *PostgresTenantRepo) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var td tenantDB
	query := `SELECT id, name, api_key, creds, risk_config, rate_limit_config, allowed_signers FROM tenants WHERE id = $1 LIMIT 1`
	err := r.db.GetContext(ctx, &td, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return r.toDomain(&td)
}

func (r *PostgresTenantRepo) Update(ctx context.Context, t *model.Tenant) error {
	creds, _ := json.Marshal(t.Creds)
	risk, _ := json.Marshal(t.Risk)
	rate, _ := json.Marshal(t.Rate)
	signers, _ := json.Marshal(t.AllowedSigners)
	_, err := r.db.ExecContext(ctx, `
		UPDATE tenants
		SET name = $2, api_key = $3, creds = $4, risk_config = $5, rate_limit_config = $6, allowed_signers = $7, updated_at = $8
		WHERE id = $1
	`, t.ID, t.Name, t.ApiKey, creds, risk, rate, signers, time.Now().UTC())
	return err
}

func (r *PostgresTenantRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	return err
}

func (r *PostgresTenantRepo) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tenants (
			id TEXT PRIMARY KEY,
			name TEXT,
			api_key TEXT UNIQUE,
			creds JSONB,
			risk_config JSONB,
			rate_limit_config JSONB,
			allowed_signers JSONB,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, `ALTER TABLE tenants ADD COLUMN IF NOT EXISTS allowed_signers JSONB`)
	_, _ = r.db.ExecContext(ctx, `ALTER TABLE tenants ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`)
	_, _ = r.db.ExecContext(ctx, `ALTER TABLE tenants ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ`)
	return nil
}
