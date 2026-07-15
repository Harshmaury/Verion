package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Sentinel errors for JWT operations.
var (
	ErrTokenExpired = errors.New("auth: token expired")
	ErrTokenInvalid = errors.New("auth: token invalid")
)

// Claims holds the payload of a Verion JWT.
type Claims struct {
	Subject   string `json:"sub"`    // identity ID
	IssuedAt  int64  `json:"iat"`    // unix timestamp
	ExpiresAt int64  `json:"exp"`    // unix timestamp
	Issuer    string `json:"iss"`    // "verion"
	TenantID  string `json:"tid"`    // tenant ID
	Handle    string `json:"handle"` // identity handle
	Type      string `json:"type"`   // identity type
}

// Valid returns an error if the claims are expired or malformed.
func (c *Claims) Valid() error {
	if time.Now().Unix() > c.ExpiresAt {
		return ErrTokenExpired
	}
	if c.Subject == "" || c.TenantID == "" {
		return ErrTokenInvalid
	}
	return nil
}

// TokenConfig holds configuration for JWT issuance.
type TokenConfig struct {
	TTL    time.Duration
	Issuer string
}

// DefaultTokenConfig returns secure defaults.
func DefaultTokenConfig() *TokenConfig {
	return &TokenConfig{
		TTL:    time.Hour,
		Issuer: "verion",
	}
}

// TokenService issues and verifies JWTs signed with Ed25519.
// The private key is generated fresh at startup and lives only in memory.
type TokenService struct {
	cfg        *TokenConfig
	privateKey ed25519.PrivateKey // server signing key — never logged, never serialized
	publicKey  ed25519.PublicKey  // used for verification
}

// NewTokenService generates a new Ed25519 server signing key using crypto/rand.
func NewTokenService(cfg *TokenConfig) (*TokenService, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &TokenService{
		cfg:        cfg,
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

// Issue creates and signs a JWT for the given identity.
// Signature verified BEFORE claims are trusted in Verify.
func (s *TokenService) Issue(claims *Claims) (string, error) {
	now := time.Now()
	claims.IssuedAt  = now.Unix()
	claims.ExpiresAt = now.Add(s.cfg.TTL).Unix()
	claims.Issuer    = s.cfg.Issuer

	// Header
	header := jwtBase64(mustJSONMarshal(map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
	}))

	// Payload
	payload := jwtBase64(mustJSONMarshal(claims))

	// Signing input: header.payload
	signingInput := header + "." + payload

	// Sign with Ed25519 — signs the raw bytes of the signing input
	sig := ed25519.Sign(s.privateKey, []byte(signingInput))

	return signingInput + "." + jwtBase64(sig), nil
}

// Verify parses and validates a JWT string.
// Signature is verified BEFORE claims are decoded.
func (s *TokenService) Verify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	signingInput := parts[0] + "." + parts[1]

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	// Verify signature FIRST — never trust unverified payload.
	if !ed25519.Verify(s.publicKey, []byte(signingInput), sig) {
		return nil, ErrTokenInvalid
	}

	// Decode claims from verified payload.
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	if err := claims.Valid(); err != nil {
		return nil, err
	}

	return &claims, nil
}

// jwtBase64 encodes bytes using base64url without padding (RFC 7515).
func jwtBase64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// mustJSONMarshal marshals v to JSON or panics.
func mustJSONMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
