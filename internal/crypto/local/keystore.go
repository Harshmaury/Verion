// NOTE: LOCAL DEV ONLY — this KeyStore stores private keys in memory.
// It provides NO persistence and NO security guarantees.
// NEVER use this implementation in production.
// Production deployments must use the Vault-backed KeyStore (Phase 6).
package local

import (
	"context"
	gocrypto "crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync" // sync.Map
)

const keyRefPrefix = "local://"

// KeyStore is a development-only in-memory private key store.
// All keys are lost when the process exits.
//
// NOTE: LOCAL DEV ONLY — do not use in production.
type KeyStore struct {
	keys sync.Map // map[keyRef]crypto.PrivateKey
}

// New returns a new local KeyStore.
func New() *KeyStore {
	return &KeyStore{}
}

// Store saves the private key in memory and returns a key_ref of the form local://<keyID>.
func (ks *KeyStore) Store(_ context.Context, keyID string, privateKey gocrypto.PrivateKey) (string, error) {
	if keyID == "" {
		return "", fmt.Errorf("local keystore: keyID must not be empty")
	}
	keyRef := keyRefPrefix + keyID
	ks.keys.Store(keyRef, privateKey)
	return keyRef, nil
}

// Sign signs data using the private key identified by keyRef.
//
// Key-type dispatch:
//   - ed25519.PrivateKey: signs data directly (Ed25519 does its own internal hashing).
//   - *ecdsa.PrivateKey:  expects data to be a SHA-256 digest (32 bytes); signs with ASN.1 DER encoding.
//
// The caller (CryptoService.Sign) is responsible for providing the correct form of data.
func (ks *KeyStore) Sign(_ context.Context, keyRef string, data []byte) ([]byte, error) {
	if !strings.HasPrefix(keyRef, keyRefPrefix) {
		return nil, fmt.Errorf("local keystore: unrecognised key_ref format: %q", keyRef)
	}

	val, ok := ks.keys.Load(keyRef)
	if !ok {
		return nil, fmt.Errorf("local keystore: key not found: %q", keyRef)
	}

	switch privKey := val.(type) {
	case ed25519.PrivateKey:
		// Ed25519 signs raw data — do NOT pre-hash.
		sig, err := privKey.Sign(rand.Reader, data, gocrypto.Hash(0))
		if err != nil {
			return nil, fmt.Errorf("local keystore: ed25519 sign: %w", err)
		}
		return sig, nil

	case *ecdsa.PrivateKey:
		// ECDSA expects a digest. Enforce that callers provide a SHA-256 hash (32 bytes).
		if len(data) != sha256.Size {
			return nil, fmt.Errorf("local keystore: ecdsa sign: data must be a 32-byte SHA-256 digest, got %d bytes", len(data))
		}
		sig, err := ecdsa.SignASN1(rand.Reader, privKey, data)
		if err != nil {
			return nil, fmt.Errorf("local keystore: ecdsa sign: %w", err)
		}
		return sig, nil

	default:
		return nil, fmt.Errorf("local keystore: unsupported key type %T", val)
	}
}

// Delete removes a private key from the in-memory store.
func (ks *KeyStore) Delete(_ context.Context, keyRef string) error {
	if !strings.HasPrefix(keyRef, keyRefPrefix) {
		return fmt.Errorf("local keystore: unrecognised key_ref format: %q", keyRef)
	}
	ks.keys.Delete(keyRef)
	return nil
}
