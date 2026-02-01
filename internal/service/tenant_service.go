package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/GoPolymarket/polygate/internal/model"
	"github.com/GoPolymarket/polygate/internal/repository"
	"gorm.io/gorm"
)

type TenantService struct {
	repo    TenantRepoCRUD
	manager *TenantManager
}

type TenantRepoCRUD interface {
	TenantRepo
	List(ctx context.Context, limit, offset int) ([]*model.Tenant, error)
	GetByID(ctx context.Context, id string) (*model.Tenant, error)
	Create(ctx context.Context, t *model.Tenant) error
	Update(ctx context.Context, t *model.Tenant) error
	Delete(ctx context.Context, id string) error
}

type TenantCreateRequest struct {
	ID             string                `json:"id" binding:"required"`
	Name           string                `json:"name"`
	APIKey         string                `json:"api_key" binding:"required"`
	AllowedSigners []string              `json:"allowed_signers"`
	Creds          model.PolymarketCreds `json:"creds" binding:"required"`
	Risk           model.RiskConfig      `json:"risk"`
	Rate           model.RateLimitConfig `json:"rate_limit"`
}

type TenantUpdateRequest struct {
	Name           *string                `json:"name"`
	APIKey         *string                `json:"api_key"`
	AllowedSigners []string               `json:"allowed_signers"`
	Creds          *model.PolymarketCreds `json:"creds"`
	Risk           *model.RiskConfig      `json:"risk"`
	Rate           *model.RateLimitConfig `json:"rate_limit"`
}

type TenantCredsUpdateRequest struct {
	Creds model.PolymarketCreds `json:"creds" binding:"required"`
}

func NewTenantService(manager *TenantManager, repo TenantRepoCRUD) *TenantService {
	return &TenantService{
		repo:    repo,
		manager: manager,
	}
}

func (s *TenantService) List(ctx context.Context, limit, offset int) ([]*model.Tenant, error) {
	if s.repo != nil {
		return s.repo.List(ctx, limit, offset)
	}
	return s.manager.ListTenants(), nil
}

func (s *TenantService) Get(ctx context.Context, id string) (*model.Tenant, error) {
	if s.repo != nil {
		tenant, err := s.repo.GetByID(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrTenantNotFound
		}
		return tenant, err
	}
	tenant, ok := s.manager.GetTenantByID(id)
	if !ok {
		return nil, repository.ErrTenantNotFound
	}
	return tenant, nil
}

func (s *TenantService) Create(ctx context.Context, req TenantCreateRequest) (*model.Tenant, error) {
	tenant := &model.Tenant{
		ID:             strings.TrimSpace(req.ID),
		Name:           req.Name,
		ApiKey:         strings.TrimSpace(req.APIKey),
		AllowedSigners: req.AllowedSigners,
		Creds:          req.Creds,
		Risk:           req.Risk,
		Rate:           req.Rate,
	}
	if tenant.ID == "" || tenant.ApiKey == "" {
		return nil, fmt.Errorf("id and api_key are required")
	}
	if s.repo != nil {
		if err := s.repo.Create(ctx, tenant); err != nil {
			return nil, err
		}
	}
	s.manager.RegisterTenant(tenant)
	return tenant, nil
}

func (s *TenantService) Update(ctx context.Context, id string, req TenantUpdateRequest) (*model.Tenant, error) {
	var tenant *model.Tenant
	if s.repo != nil {
		current, err := s.repo.GetByID(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrTenantNotFound
		}
		if err != nil {
			return nil, err
		}
		tenant = current
	} else {
		current, _ := s.manager.GetTenantByID(id)
		if current == nil {
			return nil, repository.ErrTenantNotFound
		}
		tenant = current
	}

	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.APIKey != nil && *req.APIKey != "" {
		tenant.ApiKey = *req.APIKey
	}
	if req.AllowedSigners != nil {
		tenant.AllowedSigners = req.AllowedSigners
	}
	if req.Creds != nil {
		tenant.Creds = *req.Creds
	}
	if req.Risk != nil {
		tenant.Risk = *req.Risk
	}
	if req.Rate != nil {
		tenant.Rate = *req.Rate
	}

	if s.repo != nil {
		if err := s.repo.Update(ctx, tenant); err != nil {
			return nil, err
		}
	}
	s.manager.ReplaceTenant(tenant)
	return tenant, nil
}

func (s *TenantService) Delete(ctx context.Context, id string) error {
	if s.repo != nil {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
	}
	s.manager.RemoveTenantByID(id)
	return nil
}

func (s *TenantService) UpdateCreds(ctx context.Context, id string, req TenantCredsUpdateRequest) (*model.Tenant, error) {
	var tenant *model.Tenant
	if s.repo != nil {
		current, err := s.repo.GetByID(ctx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrTenantNotFound
		}
		if err != nil {
			return nil, err
		}
		tenant = current
	} else {
		current, _ := s.manager.GetTenantByID(id)
		if current == nil {
			return nil, repository.ErrTenantNotFound
		}
		tenant = current
	}

	tenant.Creds = req.Creds

	if s.repo != nil {
		if err := s.repo.Update(ctx, tenant); err != nil {
			return nil, err
		}
	}
	s.manager.ReplaceTenant(tenant)
	return tenant, nil
}
