package identity

import (
	"context"
	"fmt"
	"regexp"
)

var slugRegexp = regexp.MustCompile(`^[a-z0-9-]+$`)

// TenantService manages tenant lifecycle.
type TenantService interface {
	// CreateTenant provisions a new tenant.
	// Returns ErrAlreadyExists if slug is taken.
	CreateTenant(ctx context.Context, input CreateTenantInput) (*Tenant, error)

	// GetTenant retrieves a tenant by ID.
	// Returns ErrTenantNotFound if not found.
	GetTenant(ctx context.Context, id string) (*Tenant, error)

	// GetTenantBySlug retrieves a tenant by slug.
	// Returns ErrTenantNotFound if not found.
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)

	// SuspendTenant suspends a tenant. All identities become inaccessible.
	// Returns ErrTenantNotFound if not found.
	// Returns ErrTenantInactive if already suspended or deactivated.
	SuspendTenant(ctx context.Context, id string) error

	// ActivateTenant restores a suspended tenant to active.
	// Returns ErrTenantNotFound if not found.
	ActivateTenant(ctx context.Context, id string) error
}

// CreateTenantInput holds validated inputs for tenant creation.
type CreateTenantInput struct {
	Name       string
	Slug       string     // must be lowercase, alphanumeric + hyphens only
	Tier       TenantTier
	DataRegion string // default "global" if empty
}

// tenantService is the unexported implementation of TenantService.
type tenantService struct {
	repos *Repositories
}

// Compile-time assertion.
var _ TenantService = (*tenantService)(nil)

// NewTenantService constructs a TenantService.
func NewTenantService(repos *Repositories) TenantService {
	return &tenantService{repos: repos}
}

func (s *tenantService) CreateTenant(ctx context.Context, input CreateTenantInput) (*Tenant, error) {
	if err := validateCreateTenantInput(input); err != nil {
		return nil, err
	}
	if input.DataRegion == "" {
		input.DataRegion = "global"
	}

	tenant := &Tenant{
		Name:       input.Name,
		Slug:       input.Slug,
		Tier:       input.Tier,
		DataRegion: input.DataRegion,
		Status:     TenantStatusActive,
	}

	created, err := s.repos.Tenants.Create(ctx, tenant)
	if err != nil {
		return nil, wrapRepoError(err, "create tenant")
	}

	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   created.ID,
		EventType:  AuditEventTenantCreated,
		EntityType: "tenant",
		EntityID:   created.ID,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("write audit event: %w", err)
	}

	return created, nil
}

func (s *tenantService) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	t, err := s.repos.Tenants.GetByID(ctx, id)
	if err != nil {
		return nil, wrapRepoError(err, "get tenant")
	}
	return t, nil
}

func (s *tenantService) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	t, err := s.repos.Tenants.GetBySlug(ctx, slug)
	if err != nil {
		return nil, wrapRepoError(err, "get tenant by slug")
	}
	return t, nil
}

func (s *tenantService) SuspendTenant(ctx context.Context, id string) error {
	t, err := s.repos.Tenants.GetByID(ctx, id)
	if err != nil {
		return wrapRepoError(err, "get tenant for suspend")
	}
	if t.Status != TenantStatusActive {
		return ErrTenantInactive
	}

	if err := s.repos.Tenants.UpdateStatus(ctx, id, TenantStatusSuspended); err != nil {
		return wrapRepoError(err, "suspend tenant")
	}

	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   id,
		EventType:  AuditEventTenantSuspended,
		EntityType: "tenant",
		EntityID:   id,
		Success:    true,
	}); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

func (s *tenantService) ActivateTenant(ctx context.Context, id string) error {
	t, err := s.repos.Tenants.GetByID(ctx, id)
	if err != nil {
		return wrapRepoError(err, "get tenant for activate")
	}
	if t.Status == TenantStatusDeactivated {
		return ErrTenantInactive
	}

	if err := s.repos.Tenants.UpdateStatus(ctx, id, TenantStatusActive); err != nil {
		return wrapRepoError(err, "activate tenant")
	}
	return nil
}

func validateCreateTenantInput(input CreateTenantInput) error {
	if input.Name == "" {
		return &ValidationError{Field: "name", Message: "must not be empty"}
	}
	if input.Slug == "" {
		return &ValidationError{Field: "slug", Message: "must not be empty"}
	}
	if !slugRegexp.MatchString(input.Slug) {
		return &ValidationError{Field: "slug", Message: "must match ^[a-z0-9-]+$"}
	}
	return nil
}
