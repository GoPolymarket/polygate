package middleware

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/gin-gonic/gin"
)

const HeaderAdminKey = "X-Admin-Key"
const HeaderAdminSecretKey = "X-Admin-Secret"

func AdminMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || cfg.Auth.AdminKey == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin key not configured"})
			c.Abort()
			return
		}
		if c.GetHeader(HeaderAdminKey) != cfg.Auth.AdminKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid admin key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func AdminSecretMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || cfg.Auth.AdminSecretKey == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin secret key not configured"})
			c.Abort()
			return
		}
		if c.GetHeader(HeaderAdminSecretKey) != cfg.Auth.AdminSecretKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid admin secret key"})
			c.Abort()
			return
		}
		c.Next()
	}
}
