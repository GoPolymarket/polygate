package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/pkg/apperrors"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

func (h *AuditHandler) List(c *gin.Context) {
	tenantVal, exists := c.Get(middleware.ContextTenantKey)
	if !exists {
		c.Error(apperrors.New(apperrors.ErrAuthFailed, "unauthorized: missing tenant context", nil))
		return
	}
	tenant := tenantVal.(*model.Tenant)

	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	var fromPtr *time.Time
	var toPtr *time.Time
	if raw := c.Query("from"); raw != "" {
		if t, err := parseTime(raw); err == nil {
			fromPtr = &t
		} else {
			c.Error(apperrors.NewInvalidRequest(err.Error()))
			return
		}
	}
	if raw := c.Query("to"); raw != "" {
		if t, err := parseTime(raw); err == nil {
			toPtr = &t
		} else {
			c.Error(apperrors.NewInvalidRequest(err.Error()))
			return
		}
	}

	records, err := h.svc.List(c.Request.Context(), tenant.ID, limit, fromPtr, toPtr)
	if err != nil {
		c.Error(apperrors.New(apperrors.ErrInternal, err.Error(), err))
		return
	}
	c.JSON(http.StatusOK, records)
}

func parseTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid time format")
}
