package middleware

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	HeaderGatewayKey = "X-Gateway-Key"
	ContextTenantKey = "tenant"
)

func AuthMiddleware(cfg *config.Config, tm *service.TenantManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader(HeaderGatewayKey)
		if apiKey == "" {
			if cfg != nil && !cfg.Auth.RequireAPIKey {
				if tenant := tm.DefaultTenant(); tenant != nil {
					c.Set(ContextTenantKey, tenant)
					c.Next()
					return
				}
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			c.Abort()
			return
		}

		tenant, ok := tm.GetTenantByApiKeyWithFallback(c.Request.Context(), apiKey)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			c.Abort()
			return
		}

		// 将租户信息存入上下文
		c.Set(ContextTenantKey, tenant)
		c.Next()
	}
}
