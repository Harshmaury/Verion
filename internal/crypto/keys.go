package crypto

import (
	"context"
	gocrypto "crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// generateEd25519Key generates an Ed25519 key pair, stores the private key
// in the KeyStore, and returns the public key DER, JWK, key_ref, and fingerprint.
func generateEd25519Key(ctx context.Context, keyID string, store KeyStore) (
	publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error,
) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}

	// DER-encode the public key.
	publicKeyDER, err = x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("marshal ed25519 public key: %w", err)
	}

	// Store private key; receive opaque key_ref.
	keyRef, err = store.Store(ctx, keyID, gocrypto.PrivateKey(privKey))
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("store ed25519 private key: %w", err)
	}

	// Build JWK for Ed25519 public key.
	// x = base64url(public key bytes, 32 bytes)
	jwkMap := map[string]string{
		"kty": "OKP",
		"crv": "Ed25519",
		"x":   base64.RawURLEncoding.EncodeToString(pubKey),
		"kid": keyID,
	}
	jwk, err = json.Marshal(jwkMap)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("marshal ed25519 jwk: %w", err)
	}

	fingerprint = fingerprintDER(publicKeyDER)
	return publicKeyDER, jwk, keyRef, fingerprint, nil
}

// generateECDSAP256Key generates an ECDSA P-256 key pair, stores the private key
// in the KeyStore, and returns the public key DER, JWK, key_ref, and fingerprint.
func generateECDSAP256Key(ctx context.Context, keyID string, store KeyStore) (
	publicKeyDER []byte, jwk []byte, keyRef string, fingerprint string, err error,
) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("generate ecdsa p256 key: %w", err)
	}

	// DER-encode the public key.
	publicKeyDER, err = x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("marshal ecdsa p256 public key: %w", err)
	}

	// Store private key; receive opaque key_ref.
	keyRef, err = store.Store(ctx, keyID, gocrypto.PrivateKey(privKey))
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("store ecdsa p256 private key: %w", err)
	}

	// Build JWK for ECDSA P-256 public key.
	// x, y = base64url of the 32-byte big-endian coordinates.
	pub := privKey.PublicKey
	byteLen := (pub.Curve.Params().BitSize + 7) / 8
	xBytes := zeroPad(pub.X.Bytes(), byteLen)
	yBytes := zeroPad(pub.Y.Bytes(), byteLen)

	jwkMap := map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(xBytes),
		"y":   base64.RawURLEncoding.EncodeToString(yBytes),
		"kid": keyID,
	}
	jwk, err = json.Marshal(jwkMap)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("marshal ecdsa p256 jwk: %w", err)
	}

	fingerprint = fingerprintDER(publicKeyDER)
	return publicKeyDER, jwk, keyRef, fingerprint, nil
}

// fingerprintDER computes the SHA-256 fingerprint of a DER-encoded public key.
// Returns a lowercase hex string (64 characters).
func fingerprintDER(publicKeyDER []byte) string {
	sum := sha256.Sum256(publicKeyDER)
	return fmt.Sprintf("%x", sum)
}

// zeroPad left-pads b with zeros to length n.
func zeroPad(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	padded := make([]byte, n)
	copy(padded[n-len(b):], b)
	return padded
}
