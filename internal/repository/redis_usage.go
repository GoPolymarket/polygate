package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type RedisUsageRepo struct {
	client *RedisClient
	prefix string
}

func NewRedisUsageRepo(client *RedisClient) *RedisUsageRepo {
	return &RedisUsageRepo{
		client: client,
		prefix: "risk",
	}
}

func (r *RedisUsageRepo) GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error) {
	key := r.makeKey(tenantID)
	orders := 0
	volume := 0.0

	if val, err := r.client.Do(ctx, "HGET", key, "orders"); err == nil {
		if s, ok := redisString(val); ok {
			if parsed, err := strconv.Atoi(s); err == nil {
				orders = parsed
			}
		}
	}
	if val, err := r.client.Do(ctx, "HGET", key, "volume"); err == nil {
		if s, ok := redisString(val); ok {
			if parsed, err := strconv.ParseFloat(s, 64); err == nil {
				volume = parsed
			}
		}
	}
	return orders, volume, nil
}

func (r *RedisUsageRepo) AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error {
	key := r.makeKey(tenantID)
	if orders != 0 {
		if _, err := r.client.Do(ctx, "HINCRBY", key, "orders", strconv.Itoa(orders)); err != nil {
			return err
		}
	}
	if amount != 0 {
		if _, err := r.client.Do(ctx, "HINCRBYFLOAT", key, "volume", fmt.Sprintf("%f", amount)); err != nil {
			return err
		}
	}
	return nil
}

func (r *RedisUsageRepo) makeKey(tenantID string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("%s:%s:%s", r.prefix, tenantID, date)
}
