package identity

// ─────────────────────────────────────────────────────────────────────────────
// repository.go — Repository interfaces for the identity domain
//
// WHY INTERFACES?
//   The service layer depends on these interfaces, not on concrete types.
//   This means:
//     1. We can swap PostgreSQL for any other store without touching services
//     2. We can write fast in-memory fakes for unit tests
//     3. The compiler enforces the contract — if postgres_repo.go doesn't
//        implement every method here, it won't compile
//
// PATTERN:
//   Every repository has exactly one interface defined here.
//   The concrete implementation lives in internal/identity/postgres/.
// ─────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"time"
)

// ── TenantRepository ──────────────────────────────────────────────────────────

// TenantRepository defines all database operations for tenants.
type TenantRepository interface {
	// Create inserts a new tenant and returns it with generated ID and timestamps.
	Create(ctx context.Context, tenant *Tenant) (*Tenant, error)

	// GetByID retrieves a tenant by its UUID.
	// Returns ErrTenantNotFound if no tenant exists with that ID.
	GetByID(ctx context.Context, id string) (*Tenant, error)

	// GetBySlug retrieves a tenant by its unique slug.
	// Returns ErrTenantNotFound if no tenant exists with that slug.
	GetBySlug(ctx context.Context, slug string) (*Tenant, error)

	// Update persists changes to a tenant's mutable fields.
	Update(ctx context.Context, tenant *Tenant) (*Tenant, error)

	// UpdateStatus changes the operational status of a tenant.
	UpdateStatus(ctx context.Context, id string, status TenantStatus) error

	// List returns all tenants matching the given filter.
	List(ctx context.Context, filter TenantFilter) ([]*Tenant, error)
}

// TenantFilter specifies criteria for listing tenants.
type TenantFilter struct {
	Status *TenantStatus
	Tier   *TenantTier
	Limit  int
	Offset int
}

// ── IdentityRepository ────────────────────────────────────────────────────────

// IdentityRepository defines all database operations for identities.
type IdentityRepository interface {
	// Create inserts a new identity and returns it with generated ID and timestamps.
	// Automatically writes an AuditEvent for the creation.
	Create(ctx context.Context, identity *Identity) (*Identity, error)

	// GetByID retrieves an identity by its UUID within the current tenant.
	// Returns ErrNotFound if no identity exists.
	GetByID(ctx context.Context, tenantID, id string) (*Identity, error)

	// GetByHandle retrieves an identity by its handle within a tenant.
	// Returns ErrNotFound if no identity exists with that handle.
	GetByHandle(ctx context.Context, tenantID, handle string) (*Identity, error)

	// Update persists changes to an identity's mutable fields.
	// Enforces optimistic locking via the Version field.
	// Returns ErrVersionConflict if the version has changed since last read.
	Update(ctx context.Context, identity *Identity) (*Identity, error)

	// UpdateStatus changes the lifecycle status of an identity.
	// Setting status to Deactivated sets deactivated_at automatically.
	UpdateStatus(ctx context.Context, tenantID, id string, status IdentityStatus) error

	// SetPrimaryKey sets the primary_key_id for an identity.
	SetPrimaryKey(ctx context.Context, tenantID, identityID, keyID string) error

	// List returns identities matching the given filter within a tenant.
	List(ctx context.Context, tenantID string, filter IdentityFilter) ([]*Identity, error)

	// Count returns the number of identities matching the filter.
	Count(ctx context.Context, tenantID string, filter IdentityFilter) (int64, error)
}

// IdentityFilter specifies criteria for listing identities.
type IdentityFilter struct {
	Type    *IdentityType
	Status  *IdentityStatus
	Limit   int
	Offset  int
	OrderBy string // "created_at" | "updated_at" | "display_name"
	Desc    bool
}

// ── IdentityKeyRepository ─────────────────────────────────────────────────────

// IdentityKeyRepository defines all database operations for cryptographic keys.
type IdentityKeyRepository interface {
	// Create inserts a new key record.
	// Only the public key and key_ref are stored. Never the private key.
	Create(ctx context.Context, key *IdentityKey) (*IdentityKey, error)

	// GetByID retrieves a key by its UUID.
	// Returns ErrKeyNotFound if no key exists.
	GetByID(ctx context.Context, tenantID, id string) (*IdentityKey, error)

	// GetByFingerprint retrieves a key by its public key fingerprint.
	GetByFingerprint(ctx context.Context, tenantID, fingerprint string) (*IdentityKey, error)

	// GetActiveByPurpose returns the active key for a given identity and purpose.
	// Returns ErrKeyNotFound if no active key exists for that purpose.
	GetActiveByPurpose(ctx context.Context, tenantID, identityID string, purpose KeyPurpose) (*IdentityKey, error)

	// ListByIdentity returns all keys for an identity, optionally filtered by status.
	ListByIdentity(ctx context.Context, tenantID, identityID string, status *KeyStatus) ([]*IdentityKey, error)

	// Rotate marks the old key as rotated and records the successor key ID.
	// The new key must already be created before calling Rotate.
	Rotate(ctx context.Context, tenantID, oldKeyID, newKeyID string) error

	// Revoke marks a key as revoked. Revocation is permanent.
	Revoke(ctx context.Context, tenantID, keyID string) error

	// MarkCompromised marks a key as compromised and records the time.
	// This is a high-severity operation that triggers audit and alerting.
	MarkCompromised(ctx context.Context, tenantID, keyID string) error
}

// ── CredentialRepository ──────────────────────────────────────────────────────

// CredentialRepository defines database operations for credentials.
type CredentialRepository interface {
	// Create inserts a new credential with encrypted data.
	Create(ctx context.Context, cred *Credential) (*Credential, error)

	// GetByID retrieves a credential by its UUID.
	GetByID(ctx context.Context, tenantID, id string) (*Credential, error)

	// GetWebAuthnByCredentialID looks up a passkey by the authenticator-assigned ID.
	// Used during WebAuthn assertion verification.
	GetWebAuthnByCredentialID(ctx context.Context, tenantID string, credentialID []byte) (*Credential, error)

	// ListByIdentity returns all credentials for an identity.
	ListByIdentity(ctx context.Context, tenantID, identityID string, status *CredentialStatus) ([]*Credential, error)

	// UpdateSignCount updates the WebAuthn signature counter after a successful assertion.
	// The new count must be strictly greater than the current count.
	UpdateSignCount(ctx context.Context, tenantID, credentialID string, newCount int64) error

	// UpdateLastUsed records the time a credential was last successfully used.
	UpdateLastUsed(ctx context.Context, tenantID, credentialID string, at time.Time) error

	// Revoke marks a credential as revoked. Revocation is permanent.
	Revoke(ctx context.Context, tenantID, credentialID string) error
}

// ── AuditRepository ───────────────────────────────────────────────────────────

// AuditRepository defines operations for the immutable audit log.
//
// CRITICAL: This interface intentionally has NO Update or Delete methods.
// Audit events are append-only. Once written, they cannot be modified.
type AuditRepository interface {
	// Insert appends a new audit event to the log.
	// This is the ONLY write operation available on the audit log.
	Insert(ctx context.Context, event *AuditEvent) error

	// GetByID retrieves a single audit event by ID.
	GetByID(ctx context.Context, tenantID, id string) (*AuditEvent, error)

	// ListByEntity returns audit events for a specific entity, newest first.
	ListByEntity(ctx context.Context, tenantID, entityType, entityID string, limit, offset int) ([]*AuditEvent, error)

	// ListByActor returns audit events performed by a specific actor.
	ListByActor(ctx context.Context, tenantID, actorID string, limit, offset int) ([]*AuditEvent, error)

	// ListByTenant returns all audit events for a tenant, newest first.
	ListByTenant(ctx context.Context, tenantID string, filter AuditFilter) ([]*AuditEvent, error)
}

// AuditFilter specifies criteria for querying audit events.
type AuditFilter struct {
	EventType  *string
	EntityType *string
	ActorID    *string
	Success    *bool
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}

// ── RecoveryRepository ────────────────────────────────────────────────────────

// RecoveryRepository defines database operations for recovery methods.
type RecoveryRepository interface {
	// Create inserts a new recovery method with encrypted data.
	Create(ctx context.Context, method *RecoveryMethod) (*RecoveryMethod, error)

	// GetByID retrieves a recovery method by its UUID.
	GetByID(ctx context.Context, tenantID, id string) (*RecoveryMethod, error)

	// ListByIdentity returns all recovery methods for an identity.
	ListByIdentity(ctx context.Context, tenantID, identityID string) ([]*RecoveryMethod, error)

	// MarkConsumed marks a one-time recovery method as consumed.
	// Consumed methods can never be used again.
	MarkConsumed(ctx context.Context, tenantID, id string) error

	// Revoke marks a recovery method as revoked.
	Revoke(ctx context.Context, tenantID, id string) error
}

// ── Repositories (aggregate) ──────────────────────────────────────────────────

// Repositories is a container holding all repository instances.
// The service layer receives this struct and uses it to access
// all data operations through a single entry point.
type Repositories struct {
	Tenants     TenantRepository
	Identities  IdentityRepository
	Keys        IdentityKeyRepository
	Credentials CredentialRepository
	Recovery    RecoveryRepository
	Audit       AuditRepository
}
