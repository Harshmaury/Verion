package crypto

import (
	"context"
	gocrypto "crypto"
)

// KeyStore abstracts private key storage.
// Private key material never leaves the KeyStore.
// The rest of the system only ever sees public keys and key_ref strings.
type KeyStore interface {
	// Store saves a private key and returns a key_ref string.
	// The key_ref is an opaque reference — format depends on implementation.
	Store(ctx context.Context, keyID string, privateKey gocrypto.PrivateKey) (keyRef string, err error)

	// Sign signs the digest using the private key identified by keyRef.
	// The caller provides the digest (pre-hashed data), not raw data.
	// For Ed25519: pass raw data (Ed25519 handles hashing internally).
	// For ECDSA: pass SHA-256 digest.
	Sign(ctx context.Context, keyRef string, digest []byte) (signature []byte, err error)

	// Delete removes a private key from storage.
	// Called when a key is revoked or compromised.
	Delete(ctx context.Context, keyRef string) error
}

// CryptoService is the single entry point for all cryptographic operations.
type CryptoService interface {
	// GenerateEd25519Key generates an Ed25519 key pair.
	// Returns public key bytes (DER), JWK bytes, key_ref, fingerprint.
	GenerateEd25519Key(ctx context.Context, keyID string) (publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error)

	// GenerateECDSAP256Key generates an ECDSA P-256 key pair.
	// Returns public key bytes (DER), JWK bytes, key_ref, fingerprint.
	GenerateECDSAP256Key(ctx context.Context, keyID string) (publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error)

	// Encrypt encrypts plaintext using AES-256-GCM.
	// Returns ciphertext and IV. Both must be stored together.
	Encrypt(plaintext []byte) (ciphertext []byte, iv []byte, err error)

	// Decrypt decrypts AES-256-GCM ciphertext using the stored IV.
	Decrypt(ciphertext []byte, iv []byte) (plaintext []byte, err error)

	// HashRecoveryCode hashes a recovery code using Argon2id.
	// Returns the hash string in the format: $argon2id$...
	HashRecoveryCode(code string) (hash string, err error)

	// VerifyRecoveryCode verifies a recovery code against its Argon2id hash.
	VerifyRecoveryCode(code string, hash string) (valid bool, err error)

	// Fingerprint computes the SHA-256 fingerprint of a DER public key.
	// Returns lowercase hex string.
	Fingerprint(publicKeyDER []byte) string

	// Sign signs data using the private key identified by keyRef.
	// Handles Ed25519 (raw data) and ECDSA (pre-hashed) correctly via KeyStore.
	Sign(ctx context.Context, keyRef string, data []byte) (signature []byte, err error)
}

// Config holds configuration for the CryptoService.
type Config struct {
	// MasterKey is the AES-256 master encryption key (32 bytes).
	// Used for encrypting identity attributes.
	// Must be provided via environment variable — never hardcoded.
	MasterKey []byte

	// Argon2 parameters for recovery code hashing.
	Argon2Time    uint32 // default: 1
	Argon2Memory  uint32 // default: 64*1024 (64MB)
	Argon2Threads uint8  // default: 4
	Argon2KeyLen  uint32 // default: 32
}

// DefaultConfig returns secure default parameters.
func DefaultConfig(masterKey []byte) *Config {
	return &Config{
		MasterKey:     masterKey,
		Argon2Time:    1,
		Argon2Memory:  64 * 1024,
		Argon2Threads: 4,
		Argon2KeyLen:  32,
	}
}

// Service implements CryptoService.
type Service struct {
	cfg   *Config
	store KeyStore
}

// New constructs a CryptoService backed by the given KeyStore.
func New(cfg *Config, store KeyStore) *Service {
	return &Service{cfg: cfg, store: store}
}

// Compile-time assertion: *Service must implement CryptoService.
var _ CryptoService = (*Service)(nil)

// GenerateEd25519Key generates an Ed25519 key pair.
func (s *Service) GenerateEd25519Key(ctx context.Context, keyID string) (publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error) {
	return generateEd25519Key(ctx, keyID, s.store)
}

// GenerateECDSAP256Key generates an ECDSA P-256 key pair.
func (s *Service) GenerateECDSAP256Key(ctx context.Context, keyID string) (publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error) {
	return generateECDSAP256Key(ctx, keyID, s.store)
}

// Encrypt encrypts plaintext using AES-256-GCM.
func (s *Service) Encrypt(plaintext []byte) (ciphertext []byte, iv []byte, err error) {
	return aesEncrypt(s.cfg.MasterKey, plaintext)
}

// Decrypt decrypts AES-256-GCM ciphertext.
func (s *Service) Decrypt(ciphertext []byte, iv []byte) (plaintext []byte, err error) {
	return aesDecrypt(s.cfg.MasterKey, ciphertext, iv)
}

// HashRecoveryCode hashes a recovery code using Argon2id.
func (s *Service) HashRecoveryCode(code string) (hash string, err error) {
	return hashArgon2id(code, s.cfg)
}

// VerifyRecoveryCode verifies a recovery code against its Argon2id hash.
func (s *Service) VerifyRecoveryCode(code string, hash string) (valid bool, err error) {
	return verifyArgon2id(code, hash)
}

// Fingerprint computes the SHA-256 fingerprint of a DER public key.
func (s *Service) Fingerprint(publicKeyDER []byte) string {
	return fingerprintDER(publicKeyDER)
}

// Sign signs data using the private key identified by keyRef.
// Delegates to the KeyStore; the KeyStore implementation is responsible
// for dispatching correctly per key type (Ed25519 vs ECDSA).
func (s *Service) Sign(ctx context.Context, keyRef string, data []byte) (signature []byte, err error) {
	return s.store.Sign(ctx, keyRef, data)
}
