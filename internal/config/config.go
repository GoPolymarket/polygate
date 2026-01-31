package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Chain      ChainConfig      `mapstructure:"chain"`
	Polymarket PolymarketConfig `mapstructure:"polymarket"`
	Builder    BuilderConfig    `mapstructure:"builder"`
	Relayer    RelayerConfig    `mapstructure:"relayer"`
	Risk       RiskConfig       `mapstructure:"risk"`
	Tenants    []TenantConfig   `mapstructure:"tenants"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type PolymarketConfig struct {
	// User's L2 API Credentials
	ApiKey        string `mapstructure:"api_key"`
	ApiSecret     string `mapstructure:"api_secret"`
	ApiPassphrase string `mapstructure:"api_passphrase"`

	// Optional: L1 Private Key for signing/onboarding (future use)
	PrivateKey string `mapstructure:"private_key"`
}

type AuthConfig struct {
	RequireAPIKey  bool   `mapstructure:"require_api_key"`
	APIKey         string `mapstructure:"api_key"`
	AdminKey       string `mapstructure:"admin_key"`
	AdminSecretKey string `mapstructure:"admin_secret_key"`
}

type DatabaseConfig struct {
	DSN                       string `mapstructure:"dsn"`
	IdempotencyRetentionHours int    `mapstructure:"idempotency_retention_hours"`
	AuditRetentionDays        int    `mapstructure:"audit_retention_days"`
	RiskRetentionDays         int    `mapstructure:"risk_retention_days"`
	CleanupIntervalMinutes    int    `mapstructure:"cleanup_interval_minutes"`
}

type RedisConfig struct {
	Addr                  string `mapstructure:"addr"`
	Password              string `mapstructure:"password"`
	DB                    int    `mapstructure:"db"`
	IdempotencyTTLSeconds int    `mapstructure:"idempotency_ttl_seconds"`
	AuditListKey          string `mapstructure:"audit_list_key"`
	AuditListMax          int    `mapstructure:"audit_list_max"`
}

type ChainConfig struct {
	RPCURL              string `mapstructure:"rpc_url"`
	EIP1271CacheSeconds int    `mapstructure:"eip1271_cache_seconds"`
	EIP1271TimeoutMs    int    `mapstructure:"eip1271_timeout_ms"`
	EIP1271Retries      int    `mapstructure:"eip1271_retries"`
}

type BuilderConfig struct {
	// The monetized "Default" builder keys (Hardcode yours here for the open source release)
	ApiKey        string `mapstructure:"api_key"`
	ApiSecret     string `mapstructure:"api_secret"`
	ApiPassphrase string `mapstructure:"api_passphrase"`
}

type RelayerConfig struct {
	BaseURL string `mapstructure:"base_url"`
	ChainID int64  `mapstructure:"chain_id"`
}

type RiskConfig struct {
	MaxSlippage               float64  `mapstructure:"max_slippage"`                // e.g. 0.05 (5%)
	MaxOrderValue             float64  `mapstructure:"max_order_value"`             // e.g. 1000 USDC
	MaxDailyValue             float64  `mapstructure:"max_daily_value"`             // e.g. 10000 USDC
	MaxDailyOrders            int      `mapstructure:"max_daily_orders"`            // e.g. 1000 orders
	BlacklistedTokenIDs       []string `mapstructure:"blacklisted_token_ids"`       // e.g. ["123", "456"]
	AllowUnverifiedSignatures bool     `mapstructure:"allow_unverified_signatures"` // allow EIP-1271 or unknown signature types
}

type TenantConfig struct {
	ID         string           `mapstructure:"id"`
	Name       string           `mapstructure:"name"`
	APIKey     string           `mapstructure:"api_key"`
	Signers    []string         `mapstructure:"signers"`
	Polymarket PolymarketConfig `mapstructure:"polymarket"`
	Risk       RiskConfig       `mapstructure:"risk"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	// Environment variables support
	// e.g. POLYGATE_POLYMARKET_API_KEY
	viper.SetEnvPrefix("polygate")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("risk.max_slippage", 0.05)
	viper.SetDefault("auth.require_api_key", false)
	viper.SetDefault("auth.admin_key", "")
	viper.SetDefault("auth.admin_secret_key", "")
	viper.SetDefault("redis.idempotency_ttl_seconds", 86400)
	viper.SetDefault("redis.audit_list_key", "audit_logs")
	viper.SetDefault("redis.audit_list_max", 10000)
	viper.SetDefault("chain.eip1271_cache_seconds", 60)
	viper.SetDefault("chain.eip1271_timeout_ms", 5000)
	viper.SetDefault("chain.eip1271_retries", 1)
	viper.SetDefault("database.idempotency_retention_hours", 168)
	viper.SetDefault("database.audit_retention_days", 30)
	viper.SetDefault("database.risk_retention_days", 30)
	viper.SetDefault("database.cleanup_interval_minutes", 60)

	// Default Builder Credentials (YOUR KEYS GO HERE)
	// 当用户没有在配置文件里覆盖这些值时，就会使用你的 Key
	viper.SetDefault("builder.api_key", "YOUR_DEFAULT_BUILDER_KEY")
	viper.SetDefault("builder.api_secret", "YOUR_DEFAULT_BUILDER_SECRET")
	viper.SetDefault("builder.api_passphrase", "YOUR_DEFAULT_BUILDER_PASSPHRASE")
	viper.SetDefault("relayer.base_url", "https://relayer-v2.polymarket.com")
	viper.SetDefault("relayer.chain_id", 137)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No config file found, using defaults and env vars")
		} else {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
