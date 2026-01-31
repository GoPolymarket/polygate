package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

func TestUpdateCredsRequiresAdminSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			AdminKey:       "admin",
			AdminSecretKey: "secret",
		},
	}

	manager := service.NewTenantManager(&config.Config{}, nil)
	manager.RegisterTenant(&model.Tenant{
		ID:     "tenant-1",
		ApiKey: "sk-tenant-1",
		Creds:  model.PolymarketCreds{},
	})
	tenantSvc := service.NewTenantService(manager, nil)
	handler := NewTenantHandler(tenantSvc)

	router := gin.New()
	admin := router.Group("/v1/tenants")
	admin.Use(middleware.AdminMiddleware(cfg))
	admin.PUT("/:id/creds", middleware.AdminSecretMiddleware(cfg), handler.UpdateCreds)

	payload := map[string]interface{}{
		"creds": map[string]string{
			"l2_api_key":        "TENANT_KEY",
			"l2_api_secret":     "TENANT_SECRET",
			"l2_api_passphrase": "TENANT_PASS",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPut, "/v1/tenants/tenant-1/creds", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.HeaderAdminKey, "admin")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without admin secret, got %d", rec.Code)
	}

	req2 := httptest.NewRequest(http.MethodPut, "/v1/tenants/tenant-1/creds", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(middleware.HeaderAdminKey, "admin")
	req2.Header.Set(middleware.HeaderAdminSecretKey, "secret")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 with admin secret, got %d", rec2.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	creds, ok := resp["creds"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing creds in response")
	}
	if creds["l2_api_secret"] == "TENANT_SECRET" {
		t.Fatalf("expected creds to be masked in response")
	}

	tenant, ok := manager.GetTenantByID("tenant-1")
	if !ok {
		t.Fatalf("tenant not found after update")
	}
	if tenant.Creds.L2ApiSecret != "TENANT_SECRET" {
		t.Fatalf("expected tenant creds to update")
	}
}
