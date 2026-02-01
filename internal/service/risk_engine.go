package service

import (
	"context"
	"fmt"
	"time"

	"github.com/GoPolymarket/polygate/internal/market"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/pkg/metrics"
	"github.com/shopspring/decimal"
)

type UsageRepo interface {
	GetDailyUsage(ctx context.Context, tenantID string) (int, float64, error)
	AddDailyUsage(ctx context.Context, tenantID string, orders int, amount float64) error
}

type RiskEngine struct {
	repo   UsageRepo
	market *market.MarketService
}

func NewRiskEngine(repo UsageRepo, marketSvc *market.MarketService) *RiskEngine {
	return &RiskEngine{repo: repo, market: marketSvc}
}

// CheckOrder 执行下单前的所有风控检查
// 如果返回 error，则必须拒绝订单
func (e *RiskEngine) CheckOrder(ctx context.Context, tenant *model.Tenant, req model.OrderRequest) error {
	config := tenant.Risk

	// 1. 基础检查：价格合理性 (Fat Finger Check)
	if req.Price <= 0 || req.Price >= 1.0 {
		metrics.RiskRejects.WithLabelValues("price_bounds").Inc()
		return fmt.Errorf("risk reject: price %.4f out of bounds (0-1)", req.Price)
	}

	if req.Size <= 0 {
		metrics.RiskRejects.WithLabelValues("invalid_size").Inc()
		return fmt.Errorf("risk reject: size must be positive")
	}

	orderVal := req.Price * req.Size

	// 2. 单笔限额 (Max Order Value)
	if config.MaxOrderValue > 0 && orderVal > config.MaxOrderValue {
		metrics.RiskRejects.WithLabelValues("max_value").Inc()
		return fmt.Errorf("risk reject: order value %.2f exceeds limit %.2f", orderVal, config.MaxOrderValue)
	}

	// 3. 价格偏离检查 (Price Deviation / Fat Finger)
	if config.MaxSlippage > 0 && e.market != nil {
		book := e.market.GetBook(req.TokenID)
		if book != nil {
			// Stale Data Check
			if time.Since(book.LastUpdated) > 10*time.Second {
				metrics.RiskRejects.WithLabelValues("stale_data").Inc()
				return fmt.Errorf("risk reject: market data stale (>10s), cannot verify price safely")
			}

			reqPrice := decimal.NewFromFloat(req.Price)
			slippage := decimal.NewFromFloat(config.MaxSlippage)
			one := decimal.NewFromInt(1)
			
			bids, asks := book.GetCopy()

			if req.Side == "BUY" {
				if len(asks) > 0 {
					bestAsk := asks[0].Price
					maxPrice := bestAsk.Mul(one.Add(slippage))
					if reqPrice.GreaterThan(maxPrice) {
						metrics.RiskRejects.WithLabelValues("slippage").Inc()
						return fmt.Errorf("risk reject: buy price %.4f deviates too much from best ask %.4f (limit: %.4f)", 
							req.Price, bestAsk.InexactFloat64(), maxPrice.InexactFloat64())
					}
				}
			} else {
				if len(bids) > 0 {
					bestBid := bids[0].Price
					minPrice := bestBid.Mul(one.Sub(slippage))
					if reqPrice.LessThan(minPrice) {
						metrics.RiskRejects.WithLabelValues("slippage").Inc()
						return fmt.Errorf("risk reject: sell price %.4f deviates too much from best bid %.4f (limit: %.4f)",
							req.Price, bestBid.InexactFloat64(), minPrice.InexactFloat64())
					}
				}
			}
		}
	}

	// 4. 黑名单市场检查 (Restricted Markets)
	for _, restrictedID := range config.RestrictedMkts {
		if req.TokenID == restrictedID {
			metrics.RiskRejects.WithLabelValues("restricted_market").Inc()
			return fmt.Errorf("risk reject: market %s is restricted", req.TokenID)
		}
	}

	// 5. 每日限额检查 (Daily Limit)
	if config.MaxDailyValue > 0 || config.MaxDailyOrders > 0 {
		currentOrders, currentVol, err := e.repo.GetDailyUsage(ctx, tenant.ID)
		if err != nil {
			return fmt.Errorf("risk check failed: %w", err)
		}

		if config.MaxDailyValue > 0 && currentVol+orderVal > config.MaxDailyValue {
			metrics.RiskRejects.WithLabelValues("daily_volume_limit").Inc()
			return fmt.Errorf("risk reject: daily volume limit exceeded (curr: %.2f, new: %.2f, max: %.2f)",
				currentVol, orderVal, config.MaxDailyValue)
		}
		if config.MaxDailyOrders > 0 && currentOrders+1 > config.MaxDailyOrders {
			metrics.RiskRejects.WithLabelValues("daily_order_limit").Inc()
			return fmt.Errorf("risk reject: daily order limit exceeded (curr: %d, max: %d)",
				currentOrders, config.MaxDailyOrders)
		}
	}

	return nil
}

// PostOrderHook 下单成功后调用，用于更新风控状态
func (e *RiskEngine) PostOrderHook(ctx context.Context, tenant *model.Tenant, req model.OrderRequest) {
	orderVal := req.Price * req.Size
	// Async or Sync? For strict limits, Sync is better but slower.
	// We'll do Sync here to ensure consistency.
	_ = e.repo.AddDailyUsage(ctx, tenant.ID, 1, orderVal)
}