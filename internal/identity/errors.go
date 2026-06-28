package identity

// ─────────────────────────────────────────────────────────────────────────────
// errors.go — Domain-specific errors for the identity package
//
// Why typed errors?
//   - The service layer can return specific errors
//   - The gRPC/HTTP layer maps them to correct status codes
//   - Callers can use errors.Is() for precise error handling
//   - No string comparison, no stringly-typed error checking
// ─────────────────────────────────────────────────────────────────────────────

import "errors"

// ── Sentinel errors ───────────────────────────────────────────────────────────
// Use errors.Is(err, ErrXxx) to check for these in callers.

var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("identity: not found")

	// ErrAlreadyExists is returned when creating a duplicate entity.
	ErrAlreadyExists = errors.New("identity: already exists")

	// ErrInvalidInput is returned when input validation fails.
	ErrInvalidInput = errors.New("identity: invalid input")

	// ErrVersionConflict is returned when optimistic locking fails.
	// The caller should re-fetch the entity and retry.
	ErrVersionConflict = errors.New("identity: version conflict")

	// ErrIdentityInactive is returned when operating on a non-active identity.
	ErrIdentityInactive = errors.New("identity: identity is not active")

	// ErrIdentityTerminal is returned when an identity is deactivated or archived.
	// Terminal identities cannot be reactivated.
	ErrIdentityTerminal = errors.New("identity: identity is in a terminal state")

	// ErrKeyNotFound is returned when a specific key does not exist.
	ErrKeyNotFound = errors.New("identity: key not found")

	// ErrKeyNotUsable is returned when a key is rotated, revoked, or expired.
	ErrKeyNotUsable = errors.New("identity: key is not usable")

	// ErrKeyCompromised is returned when operating with a compromised key.
	// Triggers incident response flow.
	ErrKeyCompromised = errors.New("identity: key is compromised")

	// ErrNoPrimaryKey is returned when an identity has no primary signing key.
	ErrNoPrimaryKey = errors.New("identity: no primary key set")

	// ErrCredentialNotFound is returned when a credential does not exist.
	ErrCredentialNotFound = errors.New("identity: credential not found")

	// ErrCredentialInactive is returned when using a revoked or expired credential.
	ErrCredentialInactive = errors.New("identity: credential is not active")

	// ErrCredentialConsumed is returned when a one-time credential is reused.
	ErrCredentialConsumed = errors.New("identity: credential already consumed")

	// ErrRecoveryNotFound is returned when a recovery method does not exist.
	ErrRecoveryNotFound = errors.New("identity: recovery method not found")

	// ErrRecoveryNotUsable is returned when a recovery method cannot be used.
	ErrRecoveryNotUsable = errors.New("identity: recovery method is not usable")

	// ErrTenantNotFound is returned when a tenant does not exist.
	ErrTenantNotFound = errors.New("identity: tenant not found")

	// ErrTenantInactive is returned when a tenant is suspended or deactivated.
	ErrTenantInactive = errors.New("identity: tenant is not active")

	// ErrUnauthorized is returned when an actor lacks permission for an operation.
	ErrUnauthorized = errors.New("identity: unauthorized")

	// ErrEncryption is returned when encryption or decryption fails.
	ErrEncryption = errors.New("identity: encryption failure")

	// ErrInvalidHandle is returned when a handle contains invalid characters.
	ErrInvalidHandle = errors.New("identity: invalid handle format")

	// ErrHandleTaken is returned when a handle is already in use within a tenant.
	ErrHandleTaken = errors.New("identity: handle already taken")
)

// ── Error wrapping helpers ────────────────────────────────────────────────────

// NotFoundError wraps ErrNotFound with entity context.
// Use when you want callers to errors.Is(err, ErrNotFound) AND
// also get a descriptive message from err.Error().
type NotFoundError struct {
	EntityType string
	ID         string
}

func (e *NotFoundError) Error() string {
	return "identity: " + e.EntityType + " not found: " + e.ID
}

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// ValidationError wraps ErrInvalidInput with field-level context.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "identity: validation failed on field '" + e.Field + "': " + e.Message
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// VersionConflictError wraps ErrVersionConflict with context.
type VersionConflictError struct {
	EntityType      string
	ID              string
	ExpectedVersion int64
	ActualVersion   int64
}

func (e *VersionConflictError) Error() string {
	return "identity: version conflict on " + e.EntityType + " " + e.ID
}

func (e *VersionConflictError) Is(target error) bool {
	return target == ErrVersionConflict
}
