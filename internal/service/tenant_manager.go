package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polymarket-go-sdk"
	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
	"golang.org/x/time/rate"
)

// TenantManager 管理租户信息、SDK 客户端实例以及限流器
type TenantManager struct {
	mu            sync.RWMutex
	tenants       map[string]*model.Tenant      // Key: Gateway ApiKey
	clients       map[string]*polymarket.Client // Key: TenantID
	limiters      map[string]*rate.Limiter      // Key: TenantID
	config        *config.Config
	defaultTenant *model.Tenant
	repo          TenantRepo
}

type TenantRepo interface {
	GetByApiKey(ctx context.Context, apiKey string) (*model.Tenant, error)
}

func NewTenantManager(cfg *config.Config, repo TenantRepo) *TenantManager {
	tm := &TenantManager{
		tenants:  make(map[string]*model.Tenant),
		clients:  make(map[string]*polymarket.Client),
		limiters: make(map[string]*rate.Limiter),
		config:   cfg,
		repo:     repo,
	}

	// 配置化租户 (优先)
	if len(cfg.Tenants) > 0 {
		for _, tenantCfg := range cfg.Tenants {
			tenant := &model.Tenant{
				ID:             tenantCfg.ID,
				Name:           tenantCfg.Name,
				ApiKey:         tenantCfg.APIKey,
				AllowedSigners: tenantCfg.Signers,
				Creds: model.PolymarketCreds{
					L2ApiKey:        tenantCfg.Polymarket.ApiKey,
					L2ApiSecret:     tenantCfg.Polymarket.ApiSecret,
					L2ApiPassphrase: tenantCfg.Polymarket.ApiPassphrase,
					PrivateKey:      tenantCfg.Polymarket.PrivateKey,
				},
				Risk: model.RiskConfig{
					MaxOrderValue:             chooseFloat(cfg.Risk.MaxOrderValue, tenantCfg.Risk.MaxOrderValue),
					MaxDailyValue:             chooseFloat(cfg.Risk.MaxDailyValue, tenantCfg.Risk.MaxDailyValue),
					MaxDailyOrders:            chooseInt(cfg.Risk.MaxDailyOrders, tenantCfg.Risk.MaxDailyOrders),
					MaxSlippage:               chooseFloat(cfg.Risk.MaxSlippage, tenantCfg.Risk.MaxSlippage),
					RestrictedMkts:            chooseStringSlice(cfg.Risk.BlacklistedTokenIDs, tenantCfg.Risk.BlacklistedTokenIDs),
					AllowUnverifiedSignatures: cfg.Risk.AllowUnverifiedSignatures || tenantCfg.Risk.AllowUnverifiedSignatures,
				},
				Rate: model.RateLimitConfig{
					QPS:   10,
					Burst: 20,
				},
			}
			tm.RegisterTenant(tenant)
		}
		return tm
	}

	// 初始化默认租户（兼容单租户模式）
	if cfg.Polymarket.ApiKey != "" || cfg.Auth.APIKey != "" {
		defaultTenant := &model.Tenant{
			ID:     "default-tenant",
			Name:   "Default User",
			ApiKey: cfg.Auth.APIKey,
			Creds: model.PolymarketCreds{
				L2ApiKey:        cfg.Polymarket.ApiKey,
				L2ApiSecret:     cfg.Polymarket.ApiSecret,
				L2ApiPassphrase: cfg.Polymarket.ApiPassphrase,
				PrivateKey:      cfg.Polymarket.PrivateKey,
			},
			Risk: model.RiskConfig{
				MaxOrderValue:             cfg.Risk.MaxOrderValue,
				MaxDailyValue:             cfg.Risk.MaxDailyValue,
				MaxDailyOrders:            cfg.Risk.MaxDailyOrders,
				MaxSlippage:               cfg.Risk.MaxSlippage,
				RestrictedMkts:            cfg.Risk.BlacklistedTokenIDs,
				AllowUnverifiedSignatures: cfg.Risk.AllowUnverifiedSignatures,
			},
			Rate: model.RateLimitConfig{
				QPS:   10, // 默认 10 QPS
				Burst: 20,
			},
		}
		if defaultTenant.ApiKey == "" {
			defaultTenant.ApiKey = "sk-default-12345"
		}
		tm.RegisterTenant(defaultTenant)
		tm.defaultTenant = defaultTenant
	}

	return tm
}

func (tm *TenantManager) RegisterTenant(t *model.Tenant) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if t == nil {
		return
	}
	tm.tenants[t.ApiKey] = t

	// 初始化限流器
	// 如果配置为0，给予一个默认的宽松限制，或者在中间件中处理
	limit := rate.Limit(t.Rate.QPS)
	if limit == 0 {
		limit = rate.Inf
	}
	burst := t.Rate.Burst
	if burst == 0 {
		burst = 1
	}
	tm.limiters[t.ID] = rate.NewLimiter(limit, burst)
}

func (tm *TenantManager) ReplaceTenant(t *model.Tenant) {
	tm.RemoveTenantByID(t.ID)
	tm.RegisterTenant(t)
}

func (tm *TenantManager) RemoveTenantByID(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	for key, tenant := range tm.tenants {
		if tenant != nil && tenant.ID == id {
			delete(tm.tenants, key)
			delete(tm.limiters, tenant.ID)
			delete(tm.clients, tenant.ID)
		}
	}
}

func (tm *TenantManager) GetTenantByID(id string) (*model.Tenant, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, tenant := range tm.tenants {
		if tenant != nil && tenant.ID == id {
			return tenant, true
		}
	}
	return nil, false
}

func (tm *TenantManager) ListTenants() []*model.Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	results := make([]*model.Tenant, 0, len(tm.tenants))
	seen := make(map[string]struct{})
	for _, tenant := range tm.tenants {
		if tenant == nil {
			continue
		}
		if _, ok := seen[tenant.ID]; ok {
			continue
		}
		seen[tenant.ID] = struct{}{}
		results = append(results, tenant)
	}
	return results
}

func (tm *TenantManager) GetTenantByApiKey(apiKey string) (*model.Tenant, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tenants[apiKey]
	return t, ok
}

func (tm *TenantManager) GetTenantByApiKeyWithFallback(ctx context.Context, apiKey string) (*model.Tenant, bool) {
	if t, ok := tm.GetTenantByApiKey(apiKey); ok {
		return t, true
	}
	if tm.repo == nil {
		return nil, false
	}
	t, err := tm.repo.GetByApiKey(ctx, apiKey)
	if err != nil || t == nil {
		return nil, false
	}
	tm.RegisterTenant(t)
	return t, true
}

func (tm *TenantManager) DefaultTenant() *model.Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.defaultTenant
}

// GetLimiterForTenant 获取租户的限流器
func (tm *TenantManager) GetLimiterForTenant(tenantID string) *rate.Limiter {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.limiters[tenantID]
}

// GetClientForTenant 获取或懒加载租户的 SDK Client
func (tm *TenantManager) GetClientForTenant(t *model.Tenant) (*polymarket.Client, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if client, ok := tm.clients[t.ID]; ok {
		return client, nil
	}

	clientOpts := []polymarket.Option{
		polymarket.WithUseServerTime(true),
	}

	if tm.config.Builder.ApiKey != "" {
		clientOpts = append(clientOpts, polymarket.WithBuilderAttribution(
			tm.config.Builder.ApiKey,
			tm.config.Builder.ApiSecret,
			tm.config.Builder.ApiPassphrase,
		))
	}

	client := polymarket.NewClient(clientOpts...)

	if t.Creds.PrivateKey != "" {
		signer, err := auth.NewPrivateKeySigner(t.Creds.PrivateKey, 137)
		if err != nil {
			return nil, fmt.Errorf("invalid private key for tenant %s: %w", t.ID, err)
		}

		apiKey := &auth.APIKey{
			Key:        t.Creds.L2ApiKey,
			Secret:     t.Creds.L2ApiSecret,
			Passphrase: t.Creds.L2ApiPassphrase,
		}

		client = client.WithAuth(signer, apiKey)
	} else if t.Creds.Address != "" && t.Creds.L2ApiKey != "" {
		signer, err := newStaticSigner(t.Creds.Address, 137)
		if err != nil {
			return nil, fmt.Errorf("invalid signer address for tenant %s: %w", t.ID, err)
		}
		apiKey := &auth.APIKey{
			Key:        t.Creds.L2ApiKey,
			Secret:     t.Creds.L2ApiSecret,
			Passphrase: t.Creds.L2ApiPassphrase,
		}
		client = client.WithAuth(signer, apiKey)
	}

	tm.clients[t.ID] = client
	return client, nil
}

func chooseFloat(base, override float64) float64 {
	if override > 0 {
		return override
	}
	return base
}

func chooseStringSlice(base, override []string) []string {
	if len(override) > 0 {
		return override
	}
	return base
}

func chooseInt(base, override int) int {
	if override > 0 {
		return override
	}
	return base
}
