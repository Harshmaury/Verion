package identity

// ─────────────────────────────────────────────────────────────────────────────
// enums.go — Typed constants matching PostgreSQL enum definitions
//
// Every enum here has a 1:1 correspondence with a PostgreSQL enum type.
// Using typed constants prevents passing invalid string values anywhere
// in the codebase — the compiler enforces correctness.
// ─────────────────────────────────────────────────────────────────────────────

// ── Tenant ───────────────────────────────────────────────────────────────────

// TenantTier represents the service tier of a tenant.
type TenantTier string

const (
	TenantTierStandard     TenantTier = "standard"
	TenantTierProfessional TenantTier = "professional"
	TenantTierEnterprise   TenantTier = "enterprise"
)

// TenantStatus represents the operational status of a tenant.
type TenantStatus string

const (
	TenantStatusActive      TenantStatus = "active"
	TenantStatusSuspended   TenantStatus = "suspended"
	TenantStatusDeactivated TenantStatus = "deactivated"
)

// ── Identity ──────────────────────────────────────────────────────────────────

// IdentityType classifies what kind of entity an identity represents.
// Verion supports 6 identity types under a single universal model.
type IdentityType string

const (
	// IdentityTypeHuman represents a human user.
	IdentityTypeHuman IdentityType = "human"

	// IdentityTypeOrg represents an organization or enterprise tenant root.
	IdentityTypeOrg IdentityType = "org"

	// IdentityTypeDevice represents a physical or virtual device.
	IdentityTypeDevice IdentityType = "device"

	// IdentityTypeService represents a software service or API endpoint.
	IdentityTypeService IdentityType = "service"

	// IdentityTypeMachine represents an automated system, job, or pipeline.
	IdentityTypeMachine IdentityType = "machine"

	// IdentityTypeAIAgent represents an autonomous AI agent.
	IdentityTypeAIAgent IdentityType = "ai_agent"
)

// IdentityStatus represents the lifecycle state of an identity.
type IdentityStatus string

const (
	// IdentityStatusPending — created but not yet verified or activated.
	IdentityStatusPending IdentityStatus = "pending"

	// IdentityStatusActive — normal operating state.
	IdentityStatusActive IdentityStatus = "active"

	// IdentityStatusSuspended — temporarily blocked.
	IdentityStatusSuspended IdentityStatus = "suspended"

	// IdentityStatusDeactivated — permanently disabled. Never hard deleted.
	IdentityStatusDeactivated IdentityStatus = "deactivated"

	// IdentityStatusArchived — soft deleted, data retained for audit.
	IdentityStatusArchived IdentityStatus = "archived"
)

// IsActive returns true if the identity can authenticate.
func (s IdentityStatus) IsActive() bool {
	return s == IdentityStatusActive
}

// IsTerminal returns true if the identity can never return to active state.
func (s IdentityStatus) IsTerminal() bool {
	return s == IdentityStatusDeactivated || s == IdentityStatusArchived
}

// ── Cryptographic Keys ────────────────────────────────────────────────────────

// KeyType specifies the cryptographic algorithm of a key pair.
type KeyType string

const (
	// KeyTypeEd25519 — preferred: fast, compact, high security (256-bit).
	KeyTypeEd25519 KeyType = "ed25519"

	// KeyTypeECDSAP256 — FIDO2/WebAuthn standard curve.
	KeyTypeECDSAP256 KeyType = "ecdsa_p256"

	// KeyTypeECDSAP384 — higher security margin variant.
	KeyTypeECDSAP384 KeyType = "ecdsa_p384"

	// KeyTypeRSA4096 — legacy compatibility only. Avoid for new identities.
	KeyTypeRSA4096 KeyType = "rsa_4096"
)

// KeyPurpose defines what a key is authorized to do.
// One identity may hold multiple keys, each with a distinct purpose.
type KeyPurpose string

const (
	// KeyPurposeSigning — used to sign tokens, assertions, identity proofs.
	KeyPurposeSigning KeyPurpose = "signing"

	// KeyPurposeEncryption — used to encrypt sensitive identity attributes.
	KeyPurposeEncryption KeyPurpose = "encryption"

	// KeyPurposeAuthentication — used for WebAuthn / FIDO2 credential assertions.
	KeyPurposeAuthentication KeyPurpose = "authentication"

	// KeyPurposeRecovery — used during identity recovery (high ceremony required).
	KeyPurposeRecovery KeyPurpose = "recovery"
)

// KeyStatus represents the lifecycle state of a cryptographic key.
type KeyStatus string

const (
	// KeyStatusActive — key is valid and in use.
	KeyStatusActive KeyStatus = "active"

	// KeyStatusRotated — superseded by a newer key. Retained for verification.
	KeyStatusRotated KeyStatus = "rotated"

	// KeyStatusRevoked — explicitly revoked. No longer trusted.
	KeyStatusRevoked KeyStatus = "revoked"

	// KeyStatusCompromised — private key material suspected compromised.
	// Triggers immediate incident response flow.
	KeyStatusCompromised KeyStatus = "compromised"
)

// IsTrusted returns true if signatures from this key should be accepted.
func (s KeyStatus) IsTrusted() bool {
	return s == KeyStatusActive || s == KeyStatusRotated
}

// ── Credentials ───────────────────────────────────────────────────────────────

// CredentialType identifies the authentication mechanism.
type CredentialType string

const (
	// CredentialTypePasskey — FIDO2 passkey via WebAuthn (preferred).
	CredentialTypePasskey CredentialType = "passkey"

	// CredentialTypeTOTP — time-based one-time password (RFC 6238).
	CredentialTypeTOTP CredentialType = "totp"

	// CredentialTypeHardwareToken — FIDO2 roaming hardware authenticator.
	CredentialTypeHardwareToken CredentialType = "hardware_token"

	// CredentialTypeRecoveryCode — single-use emergency recovery code.
	CredentialTypeRecoveryCode CredentialType = "recovery_code"

	// CredentialTypeAPIKey — service-to-service API key.
	CredentialTypeAPIKey CredentialType = "api_key"

	// CredentialTypeMTLSCert — mutual TLS client certificate.
	CredentialTypeMTLSCert CredentialType = "mtls_cert"

	// CredentialTypeBiometric — platform biometric (Face ID, Touch ID, Windows Hello).
	CredentialTypeBiometric CredentialType = "biometric"
)

// CredentialStatus represents the usability state of a credential.
type CredentialStatus string

const (
	CredentialStatusActive  CredentialStatus = "active"
	CredentialStatusRevoked CredentialStatus = "revoked"
	CredentialStatusExpired CredentialStatus = "expired"
)

// ── Recovery ──────────────────────────────────────────────────────────────────

// RecoveryType identifies the recovery mechanism.
type RecoveryType string

const (
	// RecoveryTypeRecoveryCodes — set of single-use Argon2id-hashed codes.
	RecoveryTypeRecoveryCodes RecoveryType = "recovery_codes"

	// RecoveryTypeBackupKey — cryptographic backup key pair.
	RecoveryTypeBackupKey RecoveryType = "backup_key"

	// RecoveryTypeTrustedContact — social recovery via trusted identities.
	RecoveryTypeTrustedContact RecoveryType = "trusted_contact"

	// RecoveryTypeHardwareBackup — secondary hardware security key.
	RecoveryTypeHardwareBackup RecoveryType = "hardware_backup"
)

// RecoveryStatus represents the state of a recovery method.
type RecoveryStatus string

const (
	RecoveryStatusActive   RecoveryStatus = "active"
	RecoveryStatusConsumed RecoveryStatus = "consumed"
	RecoveryStatusRevoked  RecoveryStatus = "revoked"
	RecoveryStatusExpired  RecoveryStatus = "expired"
)

// ── Actor types (for audit log) ───────────────────────────────────────────────

// ActorType identifies what kind of entity performed an action.
type ActorType string

const (
	ActorTypeHuman   ActorType = "human"
	ActorTypeService ActorType = "service"
	ActorTypeSystem  ActorType = "system"
	ActorTypeAIAgent ActorType = "ai_agent"
)
