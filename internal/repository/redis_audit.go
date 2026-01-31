package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/GoPolymarket/polygate/internal/model"
)

type RedisAuditRepo struct {
	client  *RedisClient
	listKey string
	listMax int
}

func NewRedisAuditRepo(client *RedisClient, listKey string, listMax int) *RedisAuditRepo {
	if listKey == "" {
		listKey = "audit_logs"
	}
	if listMax <= 0 {
		listMax = 10000
	}
	return &RedisAuditRepo{
		client:  client,
		listKey: listKey,
		listMax: listMax,
	}
}

func (r *RedisAuditRepo) Insert(ctx context.Context, entry *model.AuditLog) error {
	if entry == nil {
		return nil
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = r.client.Do(ctx, "LPUSH", r.listKey, string(payload))
	if err != nil {
		return err
	}
	_, _ = r.client.Do(ctx, "LTRIM", r.listKey, "0", strconv.Itoa(r.listMax-1))
	return nil
}

func (r *RedisAuditRepo) List(ctx context.Context, tenantID string, limit int, from, to *time.Time) ([]*model.AuditLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	fetch := limit * 5
	if fetch < 100 {
		fetch = 100
	}
	if fetch > r.listMax {
		fetch = r.listMax
	}
	resp, err := r.client.Do(ctx, "LRANGE", r.listKey, "0", strconv.Itoa(fetch-1))
	if err != nil || resp == nil {
		return nil, err
	}
	items, ok := resp.([]interface{})
	if !ok {
		return nil, nil
	}
	results := make([]*model.AuditLog, 0, limit)
	for _, item := range items {
		raw, ok := redisString(item)
		if !ok {
			continue
		}
		var entry model.AuditLog
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			continue
		}
		if tenantID != "" && entry.TenantID != tenantID {
			continue
		}
		if from != nil && entry.CreatedAt.Before(*from) {
			continue
		}
		if to != nil && entry.CreatedAt.After(*to) {
			continue
		}
		results = append(results, &entry)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}
