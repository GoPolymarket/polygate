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

type OrderHandler struct {
	svc *service.GatewayService
}

func NewOrderHandler(svc *service.GatewayService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	tenantVal, exists := c.Get(middleware.ContextTenantKey)
	if !exists {
		c.Error(apperrors.New(apperrors.ErrAuthFailed, "unauthorized: missing tenant context", nil))
		return
	}
	tenant := tenantVal.(*model.Tenant)

	var req model.OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewInvalidRequest(err.Error()))
		return
	}

	resp, err := h.svc.PlaceOrder(c.Request.Context(), tenant, req)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		c.Error(mapServiceError(err))
		return
	}

	middleware.AddAuditContext(c, "status", "success")
	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) BuildTypedOrder(c *gin.Context) {
	tenantVal, exists := c.Get(middleware.ContextTenantKey)
	if !exists {
		c.Error(apperrors.New(apperrors.ErrAuthFailed, "unauthorized: missing tenant context", nil))
		return
	}
	tenant := tenantVal.(*model.Tenant)

	var req model.OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewInvalidRequest(err.Error()))
		return
	}

	resp, err := h.svc.BuildTypedOrder(c.Request.Context(), tenant, req)
	if err != nil {
		middleware.AddAuditContext(c, "error", err.Error())
		c.Error(mapServiceError(err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)
	orderID := c.Param("id")

	if orderID == "" {
		c.Error(apperrors.NewInvalidRequest("order id is required"))
		return
	}

	resp, err := h.svc.CancelOrder(c.Request.Context(), tenant, model.CancelOrderInput{ID: orderID})
	if err != nil {
		c.Error(mapServiceError(err))
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
		c.Error(mapServiceError(err))
		return
	}

	middleware.AddAuditContext(c, "action", "cancel_all")

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) GetOrderbook(c *gin.Context) {
	tokenID := c.Param("id")
	if tokenID == "" {
		c.Error(apperrors.NewInvalidRequest("token_id is required"))
		return
	}

	book := h.svc.GetOrderbook(tokenID)
	if book == nil {
		c.Error(apperrors.New(apperrors.ErrNotFound, "orderbook not found or not subscribed", nil))
		return
	}

	bids, asks := book.GetCopy()
	c.JSON(http.StatusOK, gin.H{
		"token_id":     tokenID,
		"last_updated": book.LastUpdated,
		"bids":         bids,
		"asks":         asks,
	})
}

func (h *OrderHandler) GetFills(c *gin.Context) {
	fills := h.svc.GetFills()
	c.JSON(http.StatusOK, gin.H{
		"fills": fills,
		"count": len(fills),
	})
}

func (h *OrderHandler) Panic(c *gin.Context) {
	tenant := c.MustGet(middleware.ContextTenantKey).(*model.Tenant)
	
	if err := h.svc.ActivatePanicMode(c.Request.Context(), tenant); err != nil {
		c.Error(mapServiceError(err))
		return
	}

	middleware.AddAuditContext(c, "action", "panic_mode_activated")
	c.JSON(http.StatusOK, gin.H{"status": "panic_mode_active", "message": "all trading suspended and orders cancelled"})
}

// mapServiceError maps generic errors to AppErrors based on content
func mapServiceError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "risk reject") {
		return apperrors.NewRiskReject(msg)
	}
	if strings.Contains(msg, "signature") || strings.Contains(msg, "unauthorized") {
		return apperrors.New(apperrors.ErrAuthFailed, msg, err)
	}
	if strings.Contains(msg, "nonce") {
		return apperrors.New(apperrors.ErrNonce, msg, err)
	}
	if strings.Contains(msg, "panic") {
		return apperrors.New(apperrors.ErrSystemPanic, msg, err)
	}
	return apperrors.Wrap(err)
}