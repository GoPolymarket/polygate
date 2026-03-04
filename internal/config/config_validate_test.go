package config

import "testing"

func TestValidateRejectsInsecurePlaceholderAPIKey(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			RequireAPIKey: true,
			APIKey:        "sk-default-12345",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected insecure placeholder api key to fail validation")
	}
}

func TestValidateRejectsMissingAPIKeyInSingleTenantMode(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			RequireAPIKey: true,
		},
		Polymarket: PolymarketConfig{
			ApiKey: "pm-api-key",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected missing auth.api_key to fail validation in single-tenant mode")
	}
}

func TestValidateAllowsTenantModeWithoutGlobalAPIKey(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			RequireAPIKey: true,
		},
		Tenants: []TenantConfig{
			{ID: "tenant-a", APIKey: "tenant-key-a"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected tenant mode config to be valid, got: %v", err)
	}
}

func TestValidateAllowsNoAPIKeyWhenDisabled(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			RequireAPIKey: false,
		},
		Polymarket: PolymarketConfig{
			ApiKey: "pm-api-key",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected require_api_key=false config to be valid, got: %v", err)
	}
}
