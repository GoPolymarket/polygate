package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"github.com/GoPolymarket/polygate/internal/middleware"
)

type RedisIdempotencyStore struct {
	client *RedisClient
	ttl    time.Duration
	prefix string
}

func NewRedisIdempotencyStore(client *RedisClient, ttl time.Duration) *RedisIdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &RedisIdempotencyStore{
		client: client,
		ttl:    ttl,
		prefix: "idem:",
	}
}

func (s *RedisIdempotencyStore) GetOrLock(key string) (*middleware.IdempotencyRecord, bool) {
	ctx := context.Background()
	record := middleware.IdempotencyRecord{
		Status:     0,
		Body:       nil,
		CreatedAt:  time.Now().UTC(),
		Processing: true,
	}
	payload := encodeIdemRecord(record)
	resp, err := s.client.Do(ctx, "SET", s.prefix+key, payload, "NX", "PX", ttlMillis(s.ttl))
	if err == nil {
		if ok := redisOK(resp); ok {
			return nil, false
		}
	}
	val, err := s.client.Do(ctx, "GET", s.prefix+key)
	if err != nil || val == nil {
		return nil, false
	}
	str, ok := redisString(val)
	if !ok {
		return nil, false
	}
	rec, err := decodeIdemRecord(str)
	if err != nil {
		return nil, false
	}
	return rec, true
}

func (s *RedisIdempotencyStore) Save(key string, status int, body []byte) {
	ctx := context.Background()
	record := middleware.IdempotencyRecord{
		Status:     status,
		Body:       body,
		CreatedAt:  time.Now().UTC(),
		Processing: false,
	}
	payload := encodeIdemRecord(record)
	_, _ = s.client.Do(ctx, "SET", s.prefix+key, payload, "PX", ttlMillis(s.ttl))
}

func (s *RedisIdempotencyStore) Unlock(key string) {
	ctx := context.Background()
	_, _ = s.client.Do(ctx, "DEL", s.prefix+key)
}

func encodeIdemRecord(rec middleware.IdempotencyRecord) string {
	wire := map[string]interface{}{
		"status":     rec.Status,
		"body":       base64.StdEncoding.EncodeToString(rec.Body),
		"created_at": rec.CreatedAt.Unix(),
		"processing": rec.Processing,
	}
	data, _ := json.Marshal(wire)
	return string(data)
}

func decodeIdemRecord(raw string) (*middleware.IdempotencyRecord, error) {
	var wire struct {
		Status     int    `json:"status"`
		Body       string `json:"body"`
		CreatedAt  int64  `json:"created_at"`
		Processing bool   `json:"processing"`
	}
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return nil, err
	}
	body, _ := base64.StdEncoding.DecodeString(wire.Body)
	return &middleware.IdempotencyRecord{
		Status:     wire.Status,
		Body:       body,
		CreatedAt:  time.Unix(wire.CreatedAt, 0).UTC(),
		Processing: wire.Processing,
	}, nil
}

func redisOK(resp interface{}) bool {
	val, ok := redisString(resp)
	return ok && val == "OK"
}

func redisString(resp interface{}) (string, bool) {
	switch v := resp.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	default:
		return "", false
	}
}

func ttlMillis(ttl time.Duration) string {
	return strconv.FormatInt(int64(ttl/time.Millisecond), 10)
}
