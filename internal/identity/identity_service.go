package identity

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Harshmaury/verion/internal/crypto"
)

// IdentityService manages identity lifecycle.
type IdentityService interface {
	// CreateIdentity creates a new identity and generates its first key pair.
	// Returns ErrHandleTaken if handle already exists in tenant.
	// Returns ErrTenantNotFound if tenant does not exist.
	// Returns ErrTenantInactive if tenant is not active.
	CreateIdentity(ctx context.Context, input CreateIdentityInput) (*IdentityWithKeys, error)

	// GetIdentity retrieves an identity by ID within a tenant.
	// Returns ErrNotFound if not found.
	GetIdentity(ctx context.Context, tenantID, id string) (*Identity, error)

	// GetIdentityByHandle retrieves an identity by handle within a tenant.
	// Returns ErrNotFound if not found.
	GetIdentityByHandle(ctx context.Context, tenantID, handle string) (*Identity, error)

	// ListIdentities returns identities matching the filter.
	ListIdentities(ctx context.Context, tenantID string, filter IdentityFilter) ([]*Identity, error)

	// UpdateIdentity updates mutable fields on an identity.
	// Enforces optimistic locking via input.Version.
	// Returns ErrVersionConflict if version does not match.
	UpdateIdentity(ctx context.Context, input UpdateIdentityInput) (*Identity, error)

	// DeactivateIdentity permanently deactivates an identity.
	// Returns ErrIdentityTerminal if already deactivated or archived.
	DeactivateIdentity(ctx context.Context, tenantID, id string, actorID string) error

	// SuspendIdentity temporarily suspends an identity.
	// Returns ErrIdentityTerminal if already deactivated or archived.
	SuspendIdentity(ctx context.Context, tenantID, id string, actorID string) error

	// ReactivateIdentity restores a suspended identity to active.
	// Returns ErrIdentityTerminal if deactivated or archived.
	ReactivateIdentity(ctx context.Context, tenantID, id string, actorID string) error
}

// CreateIdentityInput holds validated inputs for identity creation.
type CreateIdentityInput struct {
	TenantID    string
	Type        IdentityType
	DisplayName string
	Handle      string
	CreatedBy   *string        // nil for system-created
	Attributes  map[string]any // encrypted before storage
}

// UpdateIdentityInput holds inputs for identity updates.
type UpdateIdentityInput struct {
	TenantID    string
	ID          string
	DisplayName string
	Attributes  map[string]any
	Version     int64   // optimistic locking
	ActorID     *string
}

// identityService is the unexported implementation of IdentityService.
type identityService struct {
	repos  *Repositories
	crypto crypto.CryptoService
}

// Compile-time assertion.
var _ IdentityService = (*identityService)(nil)

// NewIdentityService constructs an IdentityService.
func NewIdentityService(repos *Repositories, cryptoSvc crypto.CryptoService) IdentityService {
	return &identityService{repos: repos, crypto: cryptoSvc}
}

// CreateIdentity implements the 9-step flow from SPEC-006.
func (s *identityService) CreateIdentity(ctx context.Context, input CreateIdentityInput) (*IdentityWithKeys, error) {
	// Step 1 — Validate input.
	if err := validateCreateIdentityInput(input); err != nil {
		return nil, err
	}

	// Step 2 — Verify tenant exists and is active.
	tenant, err := s.repos.Tenants.GetByID(ctx, input.TenantID)
	if err != nil {
		return nil, wrapRepoError(err, "get tenant")
	}
	if tenant.Status != TenantStatusActive {
		return nil, ErrTenantInactive
	}

	// Encrypt attributes before storage.
	// Identity.Attributes holds the raw AES-256-GCM ciphertext (IV prepended).
	encryptedAttrs, err := s.encryptAttributes(input.Attributes)
	if err != nil {
		return nil, fmt.Errorf("encrypt attributes: %w", err)
	}

	// Step 3 — Create identity with status=pending.
	identity := &Identity{
		TenantID:    input.TenantID,
		Type:        input.Type,
		DisplayName: input.DisplayName,
		Handle:      input.Handle,
		Status:      IdentityStatusPending,
		CreatedBy:   input.CreatedBy,
		Attributes:  encryptedAttrs,
	}

	created, err := s.repos.Identities.Create(ctx, identity)
	if err != nil {
		return nil, wrapRepoError(err, "create identity")
	}

	// Step 4 — Generate Ed25519 key pair.
	pubDER, pubJWK, keyRef, fingerprint, err := s.crypto.GenerateEd25519Key(ctx, created.ID)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}

	// Step 5 — Build IdentityKey and persist.
	key := &IdentityKey{
		TenantID:     input.TenantID,
		IdentityID:   created.ID,
		KeyType:      KeyTypeEd25519,
		Purpose:      KeyPurposeSigning,
		Algorithm:    "EdDSA",
		PublicKey:    pubDER,
		PublicKeyJWK: pubJWK,
		KeyRef:       keyRef,
		Fingerprint:  fingerprint,
		Status:       KeyStatusActive,
	}

	createdKey, err := s.repos.Keys.Create(ctx, key)
	if err != nil {
		return nil, wrapRepoError(err, "create identity key")
	}

	// Step 6 — Set primary key on the identity.
	if err := s.repos.Identities.SetPrimaryKey(ctx, input.TenantID, created.ID, createdKey.ID); err != nil {
		return nil, wrapRepoError(err, "set primary key")
	}
	created.PrimaryKeyID = &createdKey.ID

	// Step 7 — Activate the identity.
	if err := s.repos.Identities.UpdateStatus(ctx, input.TenantID, created.ID, IdentityStatusActive); err != nil {
		return nil, wrapRepoError(err, "activate identity")
	}
	created.Status = IdentityStatusActive

	// Step 8 — Write audit event.
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   input.TenantID,
		EntityType: "identity",
		EntityID:   created.ID,
		ActorID:    input.CreatedBy,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("write audit event: %w", err)
	}

	// Step 9 — Return IdentityWithKeys.
	return &IdentityWithKeys{
		Identity: created,
		Keys:     []*IdentityKey{createdKey},
	}, nil
}

func (s *identityService) GetIdentity(ctx context.Context, tenantID, id string) (*Identity, error) {
	identity, err := s.repos.Identities.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, wrapRepoError(err, "get identity")
	}
	if err := s.decryptAttributes(identity); err != nil {
		return nil, fmt.Errorf("decrypt attributes: %w", err)
	}
	return identity, nil
}

func (s *identityService) GetIdentityByHandle(ctx context.Context, tenantID, handle string) (*Identity, error) {
	identity, err := s.repos.Identities.GetByHandle(ctx, tenantID, handle)
	if err != nil {
		return nil, wrapRepoError(err, "get identity by handle")
	}
	if err := s.decryptAttributes(identity); err != nil {
		return nil, fmt.Errorf("decrypt attributes: %w", err)
	}
	return identity, nil
}

func (s *identityService) ListIdentities(ctx context.Context, tenantID string, filter IdentityFilter) ([]*Identity, error) {
	identities, err := s.repos.Identities.List(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapRepoError(err, "list identities")
	}
	for _, identity := range identities {
		if err := s.decryptAttributes(identity); err != nil {
			return nil, fmt.Errorf("decrypt attributes for %s: %w", identity.ID, err)
		}
	}
	return identities, nil
}

func (s *identityService) UpdateIdentity(ctx context.Context, input UpdateIdentityInput) (*Identity, error) {
	if input.DisplayName == "" {
		return nil, &ValidationError{Field: "display_name", Message: "must not be empty"}
	}

	// Re-fetch to get current state for optimistic lock check.
	existing, err := s.repos.Identities.GetByID(ctx, input.TenantID, input.ID)
	if err != nil {
		return nil, wrapRepoError(err, "get identity for update")
	}

	encryptedAttrs, err := s.encryptAttributes(input.Attributes)
	if err != nil {
		return nil, fmt.Errorf("encrypt attributes: %w", err)
	}

	existing.DisplayName = input.DisplayName
	existing.Attributes = encryptedAttrs
	existing.Version = input.Version // repo enforces optimistic lock

	updated, err := s.repos.Identities.Update(ctx, existing)
	if err != nil {
		return nil, wrapRepoError(err, "update identity")
	}
	if err := s.decryptAttributes(updated); err != nil {
		return nil, fmt.Errorf("decrypt attributes: %w", err)
	}
	return updated, nil
}

func (s *identityService) DeactivateIdentity(ctx context.Context, tenantID, id string, actorID string) error {
	identity, err := s.repos.Identities.GetByID(ctx, tenantID, id)
	if err != nil {
		return wrapRepoError(err, "get identity for deactivate")
	}
	if identity.Status.IsTerminal() {
		return ErrIdentityTerminal
	}

	if err := s.repos.Identities.UpdateStatus(ctx, tenantID, id, IdentityStatusDeactivated); err != nil {
		return wrapRepoError(err, "deactivate identity")
	}

	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   tenantID,
		EventType:  AuditEventIdentityDeactivated,
		EntityType: "identity",
		EntityID:   id,
		ActorID:    actorPtr,
		Success:    true,
	}); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

func (s *identityService) SuspendIdentity(ctx context.Context, tenantID, id string, actorID string) error {
	identity, err := s.repos.Identities.GetByID(ctx, tenantID, id)
	if err != nil {
		return wrapRepoError(err, "get identity for suspend")
	}
	if identity.Status.IsTerminal() {
		return ErrIdentityTerminal
	}

	if err := s.repos.Identities.UpdateStatus(ctx, tenantID, id, IdentityStatusSuspended); err != nil {
		return wrapRepoError(err, "suspend identity")
	}

	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   tenantID,
		EventType:  AuditEventIdentitySuspended,
		EntityType: "identity",
		EntityID:   id,
		ActorID:    actorPtr,
		Success:    true,
	}); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

func (s *identityService) ReactivateIdentity(ctx context.Context, tenantID, id string, actorID string) error {
	identity, err := s.repos.Identities.GetByID(ctx, tenantID, id)
	if err != nil {
		return wrapRepoError(err, "get identity for reactivate")
	}
	if identity.Status.IsTerminal() {
		return ErrIdentityTerminal
	}

	if err := s.repos.Identities.UpdateStatus(ctx, tenantID, id, IdentityStatusActive); err != nil {
		return wrapRepoError(err, "reactivate identity")
	}

	var actorPtr *string
	if actorID != "" {
		actorPtr = &actorID
	}
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   tenantID,
		EventType:  AuditEventIdentityActivated,
		EntityType: "identity",
		EntityID:   id,
		ActorID:    actorPtr,
		Success:    true,
	}); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// encryptAttributes marshals attrs to JSON then encrypts with AES-256-GCM.
// The IV is prepended to the ciphertext so both are stored in Identity.Attributes.
// Returns nil if attrs is empty.
// NOTE: never log the result — may contain PII.
func (s *identityService) encryptAttributes(attrs map[string]any) ([]byte, error) {
	if len(attrs) == 0 {
		return nil, nil
	}
	plaintext, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}
	ciphertext, iv, err := s.crypto.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	// Prepend IV so a single []byte can be stored in Identity.Attributes.
	// Layout: [12 bytes IV][ciphertext...]
	combined := make([]byte, len(iv)+len(ciphertext))
	copy(combined[:len(iv)], iv)
	copy(combined[len(iv):], ciphertext)
	return combined, nil
}

// decryptAttributes decrypts Identity.Attributes into Identity.DecryptedAttributes.
// No-op if Attributes is nil.
// NOTE: never log DecryptedAttributes — may contain PII.
func (s *identityService) decryptAttributes(identity *Identity) error {
	if len(identity.Attributes) == 0 {
		return nil
	}
	const ivLen = 12
	if len(identity.Attributes) < ivLen {
		return fmt.Errorf("encrypted attributes too short")
	}
	iv := identity.Attributes[:ivLen]
	ciphertext := identity.Attributes[ivLen:]

	plaintext, err := s.crypto.Decrypt(ciphertext, iv)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	var attrs map[string]any
	if err := json.Unmarshal(plaintext, &attrs); err != nil {
		return fmt.Errorf("unmarshal attributes: %w", err)
	}
	identity.DecryptedAttributes = attrs
	return nil
}

func validateCreateIdentityInput(input CreateIdentityInput) error {
	if input.TenantID == "" {
		return &ValidationError{Field: "tenant_id", Message: "must not be empty"}
	}
	if input.Handle == "" {
		return &ValidationError{Field: "handle", Message: "must not be empty"}
	}
	if len(input.Handle) > 64 {
		return &ValidationError{Field: "handle", Message: "must be 64 characters or fewer"}
	}
	for _, r := range input.Handle {
		if r == ' ' {
			return &ValidationError{Field: "handle", Message: "must not contain spaces"}
		}
	}
	if input.DisplayName == "" {
		return &ValidationError{Field: "display_name", Message: "must not be empty"}
	}
	if len(input.DisplayName) > 128 {
		return &ValidationError{Field: "display_name", Message: "must be 128 characters or fewer"}
	}
	return nil
}
