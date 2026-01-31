package service

import (
	"context"
	"fmt"

	"github.com/GoPolymarket/polygate/internal/model"
)

type UsageRepo interface {
	GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error)
	AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error
}

type RiskEngine struct {
	repo UsageRepo
}

func NewRiskEngine(repo UsageRepo) *RiskEngine {
	return &RiskEngine{repo: repo}
}

// CheckOrder 执行下单前的所有风控检查
// 如果返回 error，则必须拒绝订单
func (e *RiskEngine) CheckOrder(ctx context.Context, tenant *model.Tenant, req OrderRequest) error {
	config := tenant.Risk

	// 1. 基础检查：价格合理性 (Fat Finger Check)
	// Polymarket 二元期权价格通常在 0.0 到 1.0 之间 (或 0-100c)
	// 这里假设输入是 0.0-1.0 的小数格式
	if req.Price <= 0 || req.Price >= 1.0 {
		return fmt.Errorf("risk reject: price %.4f out of bounds (0-1)", req.Price)
	}

	if req.Size <= 0 {
		return fmt.Errorf("risk reject: size must be positive")
	}

	orderVal := req.Price * req.Size

	// 2. 单笔限额 (Max Order Value)
	if config.MaxOrderValue > 0 && orderVal > config.MaxOrderValue {
		return fmt.Errorf("risk reject: order value %.2f exceeds limit %.2f", orderVal, config.MaxOrderValue)
	}

	// 3. 黑名单市场检查 (Restricted Markets)
	for _, restrictedID := range config.RestrictedMkts {
		if req.TokenID == restrictedID {
			return fmt.Errorf("risk reject: market %s is restricted", req.TokenID)
		}
	}

	// 4. 每日限额检查 (Daily Limit)
	if config.MaxDailyValue > 0 || config.MaxDailyOrders > 0 {
		currentOrders, currentVol, err := e.repo.GetDailyUsage(ctx, tenant.ID)
		if err != nil {
			// Fail safe: if DB error, deny trade or allow?
			// High security: deny.
			return fmt.Errorf("risk check failed: %w", err)
		}

		if config.MaxDailyValue > 0 && currentVol+orderVal > config.MaxDailyValue {
			return fmt.Errorf("risk reject: daily volume limit exceeded (curr: %.2f, new: %.2f, max: %.2f)",
				currentVol, orderVal, config.MaxDailyValue)
		}
		if config.MaxDailyOrders > 0 && currentOrders+1 > config.MaxDailyOrders {
			return fmt.Errorf("risk reject: daily order limit exceeded (curr: %d, max: %d)",
				currentOrders, config.MaxDailyOrders)
		}
	}

	return nil
}

// PostOrderHook 下单成功后调用，用于更新风控状态
func (e *RiskEngine) PostOrderHook(ctx context.Context, tenant *model.Tenant, req OrderRequest) {
	orderVal := req.Price * req.Size
	// Async or Sync? For strict limits, Sync is better but slower.
	// We'll do Sync here to ensure consistency.
	_ = e.repo.AddDailyUsage(ctx, tenant.ID, 1, orderVal)
}
