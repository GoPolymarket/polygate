package middleware

import (
	"errors"

	"github.com/GoPolymarket/polygate/internal/pkg/apperrors"
	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only handle if there are errors
		if len(c.Errors) == 0 {
			return
		}

		// Get the last error
		err := c.Errors.Last().Err
		var appErr *apperrors.AppError

		if !errors.As(err, &appErr) {
			// Unknown error, wrap as Internal
			appErr = apperrors.New(apperrors.ErrInternal, err.Error(), err)
		}

		// Log the error
		logFields := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"code", appErr.Type,
			"client_ip", c.ClientIP(),
		}

		if appErr.HTTPStatus >= 500 {
			logger.LogError(c.Request.Context(), appErr, "Internal Server Error", logFields...)
		} else {
			logger.Warn(appErr.Message, logFields...)
		}

		c.JSON(appErr.HTTPStatus, appErr)
	}
}
