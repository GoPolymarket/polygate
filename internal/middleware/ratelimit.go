package middleware

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

func RateLimitMiddleware(tm *service.TenantManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取当前租户 (必须在 AuthMiddleware 之后使用)
		tenantVal, exists := c.Get(ContextTenantKey)
		if !exists {
			// 如果没有租户信息，理论上应该由 AuthMiddleware 拦截，但为了安全起见
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		tenant := tenantVal.(*model.Tenant)

		// 2. 获取限流器
		limiter := tm.GetLimiterForTenant(tenant.ID)
		if limiter == nil {
			// 只有极其罕见的情况才会发生（TenantManager 数据不一致）
			// 这种情况下我们选择放行，或者报错，视系统策略而定
			c.Next()
			return
		}

		// 3. 尝试获取令牌
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": "1s", // 简单建议
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
