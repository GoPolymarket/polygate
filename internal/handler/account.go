package handler

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/model"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *AccountHandler) DeployProxy(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)

	// 调用 Service 执行 Gasless 部署
	addr, err := h.svc.DeployProxy(c.Request.Context(), tenant)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	middleware.AddAuditContext(c, "action", "deploy_proxy")
	middleware.AddAuditContext(c, "new_proxy", addr)

	c.JSON(http.StatusOK, gin.H{
		"status": "deployed",
		"address": addr,
	})
}
