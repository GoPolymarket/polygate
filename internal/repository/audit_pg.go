package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/jmoiron/sqlx"
)

type PostgresAuditRepo struct {
	db *sqlx.DB
}

func NewPostgresAuditRepo(db *sqlx.DB) *PostgresAuditRepo {
	repo := &PostgresAuditRepo{db: db}
	_ = repo.ensureSchema(context.Background())
	return repo
}

func (r *PostgresAuditRepo) Insert(ctx context.Context, entry *model.AuditLog) error {
	if entry == nil {
		return nil
	}
	contextJSON, _ := json.Marshal(entry.Context)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (
			id, tenant_id, method, path, ip, user_agent,
			request_body, request_header, status_code, response_body,
			latency_ms, context, created_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,
			$11,$12,$13
		)
		ON CONFLICT (id) DO NOTHING
	`, entry.ID, entry.TenantID, entry.Method, entry.Path, entry.IP, entry.UserAgent,
		entry.RequestBody, entry.RequestHeader, entry.StatusCode, entry.ResponseBody,
		entry.LatencyMs, contextJSON, entry.CreatedAt)
	return err
}

func (r *PostgresAuditRepo) List(ctx context.Context, tenantID string, limit int, from, to *time.Time) ([]*model.AuditLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `SELECT id, tenant_id, method, path, ip, user_agent, request_body, request_header, status_code, response_body, latency_ms, context, created_at FROM audit_logs`
	clauses := []string{}
	args := []interface{}{}
	idx := 1

	if tenantID != "" {
		clauses = append(clauses, fmt.Sprintf("tenant_id = $%d", idx))
		args = append(args, tenantID)
		idx++
	}
	if from != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *from)
		idx++
	}
	if to != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *to)
		idx++
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", idx)
	args = append(args, limit)

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*model.AuditLog, 0, limit)
	for rows.Next() {
		var entry model.AuditLog
		var contextJSON []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.TenantID,
			&entry.Method,
			&entry.Path,
			&entry.IP,
			&entry.UserAgent,
			&entry.RequestBody,
			&entry.RequestHeader,
			&entry.StatusCode,
			&entry.ResponseBody,
			&entry.LatencyMs,
			&contextJSON,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(contextJSON) > 0 {
			_ = json.Unmarshal(contextJSON, &entry.Context)
		} else {
			entry.Context = map[string]interface{}{}
		}
		records = append(records, &entry)
	}
	return records, nil
}

func (r *PostgresAuditRepo) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			tenant_id TEXT,
			method TEXT,
			path TEXT,
			ip TEXT,
			user_agent TEXT,
			request_body TEXT,
			request_header TEXT,
			status_code INTEGER,
			response_body TEXT,
			latency_ms BIGINT,
			context JSONB,
			created_at TIMESTAMPTZ
		)
	`)
	if err != nil {
		return err
	}
	_, _ = r.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant ON audit_logs(tenant_id, created_at DESC)`)
	return nil
}

func (r *PostgresAuditRepo) Cleanup(ctx context.Context, olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	_, err := r.db.ExecContext(ctx, `DELETE FROM audit_logs WHERE created_at < $1`, cutoff)
	return err
}
