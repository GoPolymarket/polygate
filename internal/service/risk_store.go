package service

import (
	"context"
	"sync"
	"time"
)

// RiskUsageStore 跟踪租户的实时用量（如当日交易额）
type RiskUsageStore struct {
	mu          sync.RWMutex
	dailyVolume map[string]float64 // Key: TenantID:YYYY-MM-DD
	dailyOrders map[string]int
}

func NewRiskUsageStore() *RiskUsageStore {
	return &RiskUsageStore{
		dailyVolume: make(map[string]float64),
		dailyOrders: make(map[string]int),
	}
}

func (s *RiskUsageStore) GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := s.makeKey(tenantID)
	return s.dailyOrders[key], s.dailyVolume[key], nil
}

func (s *RiskUsageStore) AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.makeKey(tenantID)
	s.dailyVolume[key] += amount
	s.dailyOrders[key] += orders
	return nil
}

func (s *RiskUsageStore) makeKey(tenantID string) string {
	// 按 UTC 日期分割
	return tenantID + ":" + time.Now().UTC().Format("2006-01-02")
}
