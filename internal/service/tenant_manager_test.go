package service

import (
	"testing"

	"github.com/GoPolymarket/polygate/internal/config"
)

func TestNewTenantManagerNoFallbackDefaultAPIKey(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			RequireAPIKey: true,
		},
		Polymarket: config.PolymarketConfig{
			ApiKey: "pm-key",
		},
	}

	tm := NewTenantManager(cfg, nil)
	if tm.DefaultTenant() != nil {
		t.Fatalf("expected no default tenant when require_api_key=true and auth.api_key is empty")
	}
	if _, ok := tm.GetTenantByApiKey("sk-default-12345"); ok {
		t.Fatalf("unexpected fallback API key tenant registration")
	}
}

func TestNewTenantManagerAllowsEmptyDefaultAPIKeyWhenAuthDisabled(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			RequireAPIKey: false,
		},
		Polymarket: config.PolymarketConfig{
			ApiKey: "pm-key",
		},
	}

	tm := NewTenantManager(cfg, nil)
	if tm.DefaultTenant() == nil {
		t.Fatalf("expected default tenant in no-auth mode")
	}
	if tm.DefaultTenant().ApiKey != "" {
		t.Fatalf("expected empty api key in no-auth mode, got %q", tm.DefaultTenant().ApiKey)
	}
}
