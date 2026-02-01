package middleware

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/pkg/apperrors"
	"github.com/gin-gonic/gin"
)

func ReadOnlyMiddleware(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		if c.Request.Method == http.MethodDelete && c.FullPath() == "/v1/panic" {
			c.Next()
			return
		}

		method := c.Request.Method
		switch method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		default:
			c.Error(apperrors.New(apperrors.ErrReadOnly, "read-only mode enabled", nil))
			c.Abort()
			return
		}
	}
}
