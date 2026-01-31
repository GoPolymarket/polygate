package handler

import (
	"net/http"

	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	svc *service.GatewayService
}

func NewOrderHandler(svc *service.GatewayService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	// 1. Get Tenant from Context (set by AuthMiddleware)
	tenantVal, exists := c.Get(middleware.ContextTenantKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing tenant context"})
		return
	}
	tenant := tenantVal.(*model.Tenant)

	// 2. Bind Request
	var req service.OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. Call Service
	resp, err := h.svc.PlaceOrder(c.Request.Context(), tenant, req)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		// In a real app, map errors to status codes (400 vs 500)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// middleware.AddAuditContext(c, "order_id", resp.OrderID)
	// middleware.AddAuditContext(c, "tx_hashes", resp.TransactionHashes)
	// TODO: Inspect resp structure to log correct fields
	middleware.AddAuditContext(c, "status", "success")

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) BuildTypedOrder(c *gin.Context) {
	tenantVal, exists := c.Get(middleware.ContextTenantKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing tenant context"})
		return
	}
	tenant := tenantVal.(*model.Tenant)

	var req service.OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.BuildTypedOrder(c.Request.Context(), tenant, req)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)
	orderID := c.Param("id")

	if orderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order id is required"})
		return
	}

	resp, err := h.svc.CancelOrder(c.Request.Context(), tenant, service.CancelOrderInput{ID: orderID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	middleware.AddAuditContext(c, "action", "cancel_order")
	middleware.AddAuditContext(c, "order_id", orderID)

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) CancelAll(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)

	resp, err := h.svc.CancelAllOrders(c.Request.Context(), tenant)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	middleware.AddAuditContext(c, "action", "cancel_all")

	c.JSON(http.StatusOK, resp)
}
