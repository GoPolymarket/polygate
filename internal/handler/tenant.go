package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

type TenantHandler struct {
	svc *service.TenantService
}

func NewTenantHandler(svc *service.TenantService) *TenantHandler {
	return &TenantHandler{svc: svc}
}

func (h *TenantHandler) List(c *gin.Context) {
	limit := 100
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}

	tenants, err := h.svc.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toTenantPublicList(tenants))
}

func (h *TenantHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	tenant, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toTenantPublic(tenant))
}

func (h *TenantHandler) Create(c *gin.Context) {
	var req service.TenantCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenant, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, toTenantPublic(tenant))
}

func (h *TenantHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	var req service.TenantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenant, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toTenantPublic(tenant))
}

func (h *TenantHandler) UpdateCreds(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	var req service.TenantCredsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tenant, err := h.svc.UpdateCreds(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toTenantPublic(tenant))
}

func (h *TenantHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *TenantHandler) GetSecret(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	tenant, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tenant)
}

type TenantPublic struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	APIKey         string                `json:"api_key"`
	AllowedSigners []string              `json:"allowed_signers,omitempty"`
	Creds          TenantCredsPublic     `json:"creds"`
	Risk           model.RiskConfig      `json:"risk"`
	Rate           model.RateLimitConfig `json:"rate_limit"`
}

type TenantCredsPublic struct {
	Address         string `json:"address"`
	L2ApiKey        string `json:"l2_api_key"`
	L2ApiSecret     string `json:"l2_api_secret"`
	L2ApiPassphrase string `json:"l2_api_passphrase"`
	PrivateKey      string `json:"private_key"`
}

func toTenantPublic(t *model.Tenant) *TenantPublic {
	if t == nil {
		return nil
	}
	return &TenantPublic{
		ID:             t.ID,
		Name:           t.Name,
		APIKey:         maskSecret(t.ApiKey),
		AllowedSigners: t.AllowedSigners,
		Creds: TenantCredsPublic{
			Address:         t.Creds.Address,
			L2ApiKey:        maskSecret(t.Creds.L2ApiKey),
			L2ApiSecret:     maskSecret(t.Creds.L2ApiSecret),
			L2ApiPassphrase: maskSecret(t.Creds.L2ApiPassphrase),
			PrivateKey:      maskSecret(t.Creds.PrivateKey),
		},
		Risk: t.Risk,
		Rate: t.Rate,
	}
}

func toTenantPublicList(tenants []*model.Tenant) []*TenantPublic {
	if len(tenants) == 0 {
		return []*TenantPublic{}
	}
	out := make([]*TenantPublic, 0, len(tenants))
	for _, tenant := range tenants {
		out = append(out, toTenantPublic(tenant))
	}
	return out
}

func maskSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
