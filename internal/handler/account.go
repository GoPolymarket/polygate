package handler

import (
	"net/http"
	"strings"

	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/pkg/apperrors"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	svc *service.AccountService
}

func NewAccountHandler(svc *service.AccountService) *AccountHandler {
	return &AccountHandler{svc: svc}
}

func (h *AccountHandler) GetProxy(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)

	status, err := h.svc.GetProxyStatus(c.Request.Context(), tenant)
	if err != nil {
		c.Error(apperrors.New(apperrors.ErrInternal, err.Error(), err))
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *AccountHandler) DeployProxy(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)

	// 调用 Service 执行 Gasless 部署
	result, err := h.svc.DeployProxy(c.Request.Context(), tenant)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		if strings.Contains(err.Error(), "private key required") {
			c.Error(apperrors.NewInvalidRequest(err.Error()))
		} else {
			c.Error(apperrors.New(apperrors.ErrUpstream, err.Error(), err))
		}
		return
	}

	middleware.AddAuditContext(c, "action", "deploy_proxy")
	if result != nil {
		if result.TransactionID != "" {
			middleware.AddAuditContext(c, "transaction_id", result.TransactionID)
		}
		if result.SafeAddress != "" {
			middleware.AddAuditContext(c, "safe_address", result.SafeAddress)
		}
	}

	var transactionID string
	var safeAddress string
	if result != nil {
		transactionID = result.TransactionID
		safeAddress = result.SafeAddress
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "submitted",
		"transaction_id": transactionID,
		"safe_address":   safeAddress,
		"address":        safeAddress,
	})
}
