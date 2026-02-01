package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(cfg *config.Config) (*RedisClient, error) {
	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("redis address is empty")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{Client: rdb}, nil
}

// Implement UsageRepo interface for Redis
func (r *RedisClient) GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error) {
	today := time.Now().Format("2006-01-02")
	keyVol := fmt.Sprintf("usage:%s:%s:volume", tenantID, today)
	keyCount := fmt.Sprintf("usage:%s:%s:count", tenantID, today)

	pipe := r.Client.Pipeline()
	volCmd := pipe.Get(ctx, keyVol)
	countCmd := pipe.Get(ctx, keyCount)
	_, err := pipe.Exec(ctx)

	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	vol, _ := volCmd.Float64()
	count, _ := countCmd.Int()

	return count, vol, nil
}

func (r *RedisClient) AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error {
	today := time.Now().Format("2006-01-02")
	keyVol := fmt.Sprintf("usage:%s:%s:volume", tenantID, today)
	keyCount := fmt.Sprintf("usage:%s:%s:count", tenantID, today)

	pipe := r.Client.Pipeline()
	// Increment
	pipe.IncrByFloat(ctx, keyVol, amount)
	pipe.IncrBy(ctx, keyCount, int64(orders))
	
	// Set Expiry (2 days is safe)
	pipe.Expire(ctx, keyVol, 48*time.Hour)
	pipe.Expire(ctx, keyCount, 48*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}
