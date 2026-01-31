package model

// RiskConfig 定义租户维度的风控规则
type RiskConfig struct {
	MaxOrderValue             float64  `json:"max_order_value"`             // 单笔最大金额 (USDC)
	MaxDailyValue             float64  `json:"max_daily_value"`             // 单日最大交易额
	MaxDailyOrders            int      `json:"max_daily_orders"`            // 单日最大订单数
	MaxSlippage               float64  `json:"max_slippage"`                // 允许的最大偏离 (0.05 = 5%)
	RestrictedMkts            []string `json:"restricted_mkts"`             // 禁止交易的市场 ID
	AllowUnverifiedSignatures bool     `json:"allow_unverified_signatures"` // 允许未验证签名
}

// RateLimitConfig 定义租户的限流规则
type RateLimitConfig struct {
	QPS   float64 `json:"qps"`   // 每秒查询数
	Burst int     `json:"burst"` // 突发桶大小
}

// PolymarketCreds 租户在 Polymarket 的交易凭证
type PolymarketCreds struct {
	Address         string `json:"address"`
	L2ApiKey        string `json:"l2_api_key"`
	L2ApiSecret     string `json:"l2_api_secret"`
	L2ApiPassphrase string `json:"l2_api_passphrase"`
	PrivateKey      string `json:"private_key"` // 实际生产中应加密存储或使用 KMS
}

// Tenant 代表一个接入方 (Bot, 客户)
type Tenant struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	ApiKey         string          `json:"api_key"` // 网关颁发给租户的 Access Key
	AllowedSigners []string        `json:"allowed_signers,omitempty"`
	Creds          PolymarketCreds `json:"creds"`
	Risk           RiskConfig      `json:"risk"`
	Rate           RateLimitConfig `json:"rate_limit"`
}
