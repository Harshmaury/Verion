package identity

import (
	"context"
	"fmt"

	"github.com/Harshmaury/verion/internal/crypto"
)

// KeyService manages cryptographic key lifecycle for identities.
type KeyService interface {
	// GenerateKey generates a new key pair for an identity.
	// Returns ErrNotFound if identity does not exist.
	// Returns ErrIdentityInactive if identity is not active.
	GenerateKey(ctx context.Context, input GenerateKeyInput) (*IdentityKey, error)

	// GetKey retrieves a key by ID.
	GetKey(ctx context.Context, tenantID, keyID string) (*IdentityKey, error)

	// ListKeys returns all keys for an identity.
	ListKeys(ctx context.Context, tenantID, identityID string) ([]*IdentityKey, error)

	// RotateKey generates a new key of the same type and purpose,
	// marks the old key as rotated, and updates primary_key_id if applicable.
	// Returns ErrKeyNotFound if oldKeyID does not exist.
	// Returns ErrKeyNotUsable if old key is already rotated/revoked/compromised.
	RotateKey(ctx context.Context, tenantID, identityID, oldKeyID string, actorID string) (*IdentityKey, error)

	// RevokeKey permanently revokes a key.
	// Returns ErrKeyNotFound if key does not exist.
	RevokeKey(ctx context.Context, tenantID, keyID string, actorID string) error
}

// GenerateKeyInput holds inputs for key generation.
type GenerateKeyInput struct {
	TenantID   string
	IdentityID string
	KeyType    KeyType    // ed25519 | ecdsa_p256
	Purpose    KeyPurpose // signing | authentication | encryption | recovery
	ActorID    *string
}

// keyService is the unexported implementation of KeyService.
type keyService struct {
	repos  *Repositories
	crypto crypto.CryptoService
}

// Compile-time assertion.
var _ KeyService = (*keyService)(nil)

// NewKeyService constructs a KeyService.
func NewKeyService(repos *Repositories, cryptoSvc crypto.CryptoService) KeyService {
	return &keyService{repos: repos, crypto: cryptoSvc}
}

func (s *keyService) GenerateKey(ctx context.Context, input GenerateKeyInput) (*IdentityKey, error) {
	identity, err := s.repos.Identities.GetByID(ctx, input.TenantID, input.IdentityID)
	if err != nil {
		return nil, wrapRepoError(err, "get identity for key generation")
	}
	if identity.Status != IdentityStatusActive {
		return nil, ErrIdentityInactive
	}

	key, err := s.generateKeyPair(ctx, input.TenantID, input.IdentityID, input.KeyType, input.Purpose)
	if err != nil {
		return nil, err
	}

	createdKey, err := s.repos.Keys.Create(ctx, key)
	if err != nil {
		return nil, wrapRepoError(err, "create key")
	}

	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   input.TenantID,
		EventType:  AuditEventKeyCreated,
		EntityType: "key",
		EntityID:   createdKey.ID,
		ActorID:    input.ActorID,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("write audit event: %w", err)
	}

	return createdKey, nil
}

func (s *keyService) GetKey(ctx context.Context, tenantID, keyID string) (*IdentityKey, error) {
	key, err := s.repos.Keys.GetByID(ctx, tenantID, keyID)
	if err != nil {
		return nil, wrapRepoError(err, "get key")
	}
	return key, nil
}

func (s *keyService) ListKeys(ctx context.Context, tenantID, identityID string) ([]*IdentityKey, error) {
	keys, err := s.repos.Keys.ListByIdentity(ctx, tenantID, identityID, nil)
	if err != nil {
		return nil, wrapRepoError(err, "list keys")
	}
	return keys, nil
}

func (s *keyService) RotateKey(ctx context.Context, tenantID, identityID, oldKeyID string, actorID string) (*IdentityKey, error) {
	oldKey, err := s.repos.Keys.GetByID(ctx, tenantID, oldKeyID)
	if err != nil {
		return nil, wrapRepoError(err, "get old key for rotation")
	}
	if !oldKey.IsUsable() {
		return nil, ErrKeyNotUsable
	}

	newKey, err := s.generateKeyPair(ctx, tenantID, identityID, oldKey.KeyType, oldKey.Purpose)
	if err != nil {
		return nil, err
	}

	createdKey, err := s.repos.Keys.Create(ctx, newKey)
	if err != nil {
		return nil, wrapRepoError(err, "create rotated key")
	}

	// Rotate marks old key as rotated and records the successor key ID.
	if err := s.repos.Keys.Rotate(ctx, tenantID, oldKeyID, createdKey.ID); err != nil {
		return nil, wrapRepoError(err, "mark old key as rotated")
	}

	// Update primary_key_id if the rotated key was primary.
	identity, err := s.repos.Identities.GetByID(ctx, tenantID, identityID)
	if err != nil {
		return nil, wrapRepoError(err, "get identity for primary key update")
	}
	if identity.PrimaryKeyID != nil && *identity.PrimaryKeyID == oldKeyID {
		if err := s.repos.Identities.SetPrimaryKey(ctx, tenantID, identityID, createdKey.ID); err != nil {
			return nil, wrapRepoError(err, "update primary key after rotation")
		}
	}

	actor := actorID
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   tenantID,
		EventType:  AuditEventKeyRotated,
		EntityType: "key",
		EntityID:   createdKey.ID,
		ActorID:    &actor,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("write audit event: %w", err)
	}

	return createdKey, nil
}

func (s *keyService) RevokeKey(ctx context.Context, tenantID, keyID string, actorID string) error {
	_, err := s.repos.Keys.GetByID(ctx, tenantID, keyID)
	if err != nil {
		return wrapRepoError(err, "get key for revocation")
	}

	if err := s.repos.Keys.Revoke(ctx, tenantID, keyID); err != nil {
		return wrapRepoError(err, "revoke key")
	}

	actor := actorID
	if err := s.repos.Audit.Insert(ctx, &AuditEvent{
		TenantID:   tenantID,
		EventType:  AuditEventKeyRevoked,
		EntityType: "key",
		EntityID:   keyID,
		ActorID:    &actor,
		Success:    true,
	}); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}

	return nil
}

// generateKeyPair calls CryptoService for the requested key type.
func (s *keyService) generateKeyPair(ctx context.Context, tenantID, identityID string, keyType KeyType, purpose KeyPurpose) (*IdentityKey, error) {
	// key ID passed to crypto store — unique within the KeyStore.
	cryptoKeyID := fmt.Sprintf("%s/%s/%s", tenantID, identityID, string(keyType))

	var (
		pubDER      []byte
		pubJWK      []byte
		keyRef      string
		fingerprint string
		algorithm   string
		err         error
	)

	switch keyType {
	case KeyTypeEd25519:
		pubDER, pubJWK, keyRef, fingerprint, err = s.crypto.GenerateEd25519Key(ctx, cryptoKeyID)
		algorithm = "EdDSA"
	case KeyTypeECDSAP256:
		pubDER, pubJWK, keyRef, fingerprint, err = s.crypto.GenerateECDSAP256Key(ctx, cryptoKeyID)
		algorithm = "ES256"
	default:
		return nil, &ValidationError{Field: "key_type", Message: fmt.Sprintf("unsupported key type: %s", keyType)}
	}
	if err != nil {
		return nil, fmt.Errorf("generate %s key: %w", keyType, err)
	}

	return &IdentityKey{
		TenantID:     tenantID,
		IdentityID:   identityID,
		KeyType:      keyType,
		Purpose:      purpose,
		Algorithm:    algorithm,
		PublicKey:    pubDER,
		PublicKeyJWK: pubJWK,
		KeyRef:       keyRef,
		Fingerprint:  fingerprint,
		Status:       KeyStatusActive,
	}, nil
}
