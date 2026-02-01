package middleware

import (
	"time"

	"github.com/GoPolymarket/polygate/internal/pkg/metrics"
	"github.com/gin-gonic/gin"
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		metrics.LatencyBucket.WithLabelValues(c.Request.URL.Path).Observe(duration)
	}
}
