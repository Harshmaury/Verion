package identity

// ─────────────────────────────────────────────────────────────────────────────
// model.go — Domain model structs for the Verion identity system
//
// These structs are the language of the entire application.
// They map 1:1 to database tables but belong to the domain layer —
// not the database layer. The repository layer translates between the two.
//
// Rules:
//   - No database-specific tags (pgx scanning handled in repository)
//   - JSON tags for API serialization
//   - Encrypted fields stored as []byte (raw ciphertext from DB)
//   - Pointer types used for nullable fields
//   - All IDs are string (UUID string representation)
// ─────────────────────────────────────────────────────────────────────────────

import "time"

// ── Tenant ────────────────────────────────────────────────────────────────────

// Tenant is the top-level isolation boundary in Verion.
// Every identity, key, credential, and audit event belongs to exactly one tenant.
// Think of a tenant as a company, organization, or isolated deployment.
type Tenant struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Slug       string       `json:"slug"`
	Tier       TenantTier   `json:"tier"`
	Status     TenantStatus `json:"status"`
	Settings   []byte       `json:"settings,omitempty"` // Raw JSONB
	DataRegion string       `json:"data_region"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsActive returns true if the tenant is operational.
func (t *Tenant) IsActive() bool {
	return t.Status == TenantStatusActive
}

// ── Identity ──────────────────────────────────────────────────────────────────

// Identity is the universal entity at the core of Verion.
// One model represents humans, organizations, devices, services,
// machines, and AI agents — unified under a single structure.
//
// Verion's foundational principle:
//   Trust is not granted; it is continuously established
//   through independently verifiable evidence.
//
// An Identity is not an account. It is a cryptographically
// bound representation of a real-world entity.
type Identity struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`

	// Type determines what kind of entity this identity represents.
	Type IdentityType `json:"type"`

	// DisplayName is the human-readable name shown in UIs.
	DisplayName string `json:"display_name"`

	// Handle is the unique identifier within a tenant (username, service name, etc.)
	Handle string `json:"handle"`

	// Status tracks where this identity is in its lifecycle.
	Status IdentityStatus `json:"status"`

	// PrimaryKeyID references the primary signing key for this identity.
	// Set after the first key is created. Nullable during initial creation.
	PrimaryKeyID *string `json:"primary_key_id,omitempty"`

	// Attributes holds type-specific data (email, phone, device model, etc.)
	// Stored encrypted (AES-256-GCM) in the database.
	// The crypto service decrypts this before returning to callers.
	Attributes []byte `json:"-"` // Never serialize raw encrypted bytes to API

	// DecryptedAttributes holds the plaintext attributes after decryption.
	// Populated by the crypto service. Never persisted.
	DecryptedAttributes map[string]any `json:"attributes,omitempty"`

	// CreatedBy is the identity that created this identity (nil for root/system).
	CreatedBy *string `json:"created_by,omitempty"`

	// Version is used for optimistic locking. Increment on every update.
	// If version in DB != version in request, reject the update.
	Version int64 `json:"version"`

	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}

// IsActive returns true if this identity can authenticate.
func (i *Identity) IsActive() bool {
	return i.Status.IsActive()
}

// IsHuman returns true if this is a human user identity.
func (i *Identity) IsHuman() bool {
	return i.Type == IdentityTypeHuman
}

// IsNonHuman returns true for device, service, machine, and AI agent identities.
func (i *Identity) IsNonHuman() bool {
	return !i.IsHuman() && i.Type != IdentityTypeOrg
}

// ── IdentityKey ───────────────────────────────────────────────────────────────

// IdentityKey represents a cryptographic key pair bound to an identity.
//
// CRITICAL SECURITY INVARIANT:
//   Private key material is NEVER stored in Verion's database.
//   KeyRef is a reference to the private key in an external secure store
//   (HashiCorp Vault, AWS KMS, hardware HSM, or local dev key store).
//   PublicKey contains only the public key — safe to store and share.
type IdentityKey struct {
	ID         string `json:"id"`
	IdentityID string `json:"identity_id"`
	TenantID   string `json:"tenant_id"`

	// KeyType identifies the cryptographic algorithm.
	KeyType KeyType `json:"key_type"`

	// Purpose defines what this key is authorized to do.
	Purpose KeyPurpose `json:"purpose"`

	// Algorithm is the signature algorithm string (e.g. "EdDSA", "ES256").
	Algorithm string `json:"algorithm"`

	// PublicKey is the DER-encoded public key. Safe to store and transmit.
	PublicKey []byte `json:"public_key"`

	// PublicKeyJWK is the JWK representation for OIDC and FIDO2 compatibility.
	PublicKeyJWK []byte `json:"public_key_jwk"`

	// KeyRef is the reference to the private key in external secure storage.
	// Format depends on the storage backend:
	//   Vault:  "vault://secret/verion/keys/<key-id>"
	//   KMS:    "arn:aws:kms:us-east-1:123456789:key/<key-id>"
	//   Local:  "local://keys/<key-id>"  (development only)
	KeyRef string `json:"-"` // Never expose key references in API responses

	// Fingerprint is the SHA-256 hash of the public key (hex-encoded).
	// Used for fast key lookup and human-readable key identification.
	Fingerprint string `json:"fingerprint"`

	Status    KeyStatus  `json:"status"`
	ValidFrom time.Time  `json:"valid_from"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`

	RotatedAt *time.Time `json:"rotated_at,omitempty"`
	RotatedTo *string    `json:"rotated_to,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// IsUsable returns true if this key can be used for cryptographic operations.
func (k *IdentityKey) IsUsable() bool {
	if !k.Status.IsTrusted() {
		return false
	}
	now := time.Now()
	if now.Before(k.ValidFrom) {
		return false
	}
	if k.ValidUntil != nil && now.After(*k.ValidUntil) {
		return false
	}
	return true
}

// ── Credential ────────────────────────────────────────────────────────────────

// Credential represents an authentication mechanism bound to an identity.
// Credentials are the proof mechanisms — passkeys, TOTP codes, hardware tokens.
// They reference keys where applicable and store encrypted credential data.
type Credential struct {
	ID         string `json:"id"`
	IdentityID string `json:"identity_id"`
	TenantID   string `json:"tenant_id"`

	// KeyID references the IdentityKey backing this credential (if key-backed).
	KeyID *string `json:"key_id,omitempty"`

	Type   CredentialType   `json:"type"`
	Status CredentialStatus `json:"status"`

	// Data is AES-256-GCM encrypted credential payload.
	// Content varies by type: TOTP secret, API key hash, cert DER, etc.
	Data   []byte `json:"-"` // Never expose raw encrypted bytes
	DataIV []byte `json:"-"`

	// WebAuthn specific fields
	AAGUID       *string `json:"aaguid,omitempty"`        // Authenticator model identifier
	CredentialID []byte  `json:"credential_id,omitempty"` // Authenticator-assigned ID
	SignCount     int64   `json:"sign_count"`              // Monotonic counter (clone detection)

	// User-facing metadata
	Name       *string `json:"name,omitempty"`        // e.g. "iPhone 15 Face ID"
	DeviceInfo []byte  `json:"device_info,omitempty"` // Platform/OS/authenticator info

	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// IsActive returns true if this credential can be used for authentication.
func (c *Credential) IsActive() bool {
	if c.Status != CredentialStatusActive {
		return false
	}
	if c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt) {
		return false
	}
	return true
}

// IsWebAuthn returns true if this credential is a passkey or hardware token.
func (c *Credential) IsWebAuthn() bool {
	return c.Type == CredentialTypePasskey || c.Type == CredentialTypeHardwareToken
}

// ── RecoveryMethod ────────────────────────────────────────────────────────────

// RecoveryMethod represents a mechanism to regain access to an identity
// when primary credentials are unavailable.
//
// Recovery must maintain equivalent security guarantees to primary authentication.
// Recovery codes are Argon2id hashed. Backup keys follow the same
// no-private-key-in-DB rule as IdentityKey.
type RecoveryMethod struct {
	ID         string `json:"id"`
	IdentityID string `json:"identity_id"`
	TenantID   string `json:"tenant_id"`

	Type   RecoveryType   `json:"type"`
	Status RecoveryStatus `json:"status"`

	// Data is AES-256-GCM encrypted recovery payload.
	// For recovery_codes: list of Argon2id-hashed codes.
	// For backup_key: public key (private key stored externally via key_ref).
	Data   []byte `json:"-"`
	DataIV []byte `json:"-"`

	// IsConsumed marks single-use recovery methods as used.
	// Once consumed, a recovery method can never be used again.
	IsConsumed bool `json:"is_consumed"`

	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// IsUsable returns true if this recovery method can be used.
func (r *RecoveryMethod) IsUsable() bool {
	if r.Status != RecoveryStatusActive {
		return false
	}
	if r.IsConsumed {
		return false
	}
	if r.ExpiresAt != nil && time.Now().After(*r.ExpiresAt) {
		return false
	}
	return true
}

// ── AuditEvent ────────────────────────────────────────────────────────────────

// AuditEvent is an immutable record of something that happened in the system.
//
// CRITICAL INVARIANT:
//   AuditEvents are NEVER updated or deleted.
//   The database enforces this via SQL RULE.
//   The repository layer enforces this by providing only an Insert method.
//   There is no Update or Delete method on the audit repository.
type AuditEvent struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`

	// What happened
	EventType  string `json:"event_type"`  // e.g. "identity.created", "key.rotated"
	EntityType string `json:"entity_type"` // "identity" | "key" | "credential" | "tenant"
	EntityID   string `json:"entity_id"`

	// Who did it
	ActorID   *string    `json:"actor_id,omitempty"`
	ActorType *ActorType `json:"actor_type,omitempty"`

	// Request context
	IPAddress *string `json:"ip_address,omitempty"`
	UserAgent *string `json:"user_agent,omitempty"`
	SessionID *string `json:"session_id,omitempty"`
	RequestID *string `json:"request_id,omitempty"`

	// Change data
	BeforeState []byte `json:"before_state,omitempty"` // Raw JSONB
	AfterState  []byte `json:"after_state,omitempty"`  // Raw JSONB
	Metadata    []byte `json:"metadata,omitempty"`     // Raw JSONB

	// Outcome
	Success   bool    `json:"success"`
	ErrorCode *string `json:"error_code,omitempty"`

	// Immutable timestamp
	OccurredAt time.Time `json:"occurred_at"`
}

// ── Commonly used composite types ─────────────────────────────────────────────

// IdentityWithKeys is a convenience struct combining an identity
// with its associated cryptographic keys. Used by the service layer
// when full identity context is needed.
type IdentityWithKeys struct {
	Identity *Identity      `json:"identity"`
	Keys     []*IdentityKey `json:"keys"`
}

// IdentityWithCredentials combines an identity with its credentials.
// Used during authentication flows.
type IdentityWithCredentials struct {
	Identity    *Identity     `json:"identity"`
	Credentials []*Credential `json:"credentials"`
}

// ── Audit event type constants ────────────────────────────────────────────────
// Standardized event type strings for the audit log.
// Always use these constants — never raw strings.

const (
	// Tenant events
	AuditEventTenantCreated     = "tenant.created"
	AuditEventTenantSuspended   = "tenant.suspended"
	AuditEventTenantDeactivated = "tenant.deactivated"

	// Identity events
	AuditEventIdentityCreated     = "identity.created"
	AuditEventIdentityActivated   = "identity.activated"
	AuditEventIdentitySuspended   = "identity.suspended"
	AuditEventIdentityDeactivated = "identity.deactivated"
	AuditEventIdentityArchived    = "identity.archived"

	// Key events
	AuditEventKeyCreated     = "key.created"
	AuditEventKeyRotated     = "key.rotated"
	AuditEventKeyRevoked     = "key.revoked"
	AuditEventKeyCompromised = "key.compromised"

	// Credential events
	AuditEventCredentialCreated  = "credential.created"
	AuditEventCredentialRevoked  = "credential.revoked"
	AuditEventCredentialExpired  = "credential.expired"

	// Authentication events
	AuditEventAuthSuccess = "auth.success"
	AuditEventAuthFailure = "auth.failure"

	// Recovery events
	AuditEventRecoveryInitiated = "recovery.initiated"
	AuditEventRecoveryCompleted = "recovery.completed"
	AuditEventRecoveryFailed    = "recovery.failed"
)
