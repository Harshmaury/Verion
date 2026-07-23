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

var (
	ErrTokenExpired = errors.New("auth: token expired")
	ErrTokenInvalid = errors.New("auth: token invalid")
)

// Claims holds the payload of a Verion JWT.
type Claims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
	TenantID  string `json:"tid"`
	Handle    string `json:"handle"`
	Type      string `json:"type"`
	SessionID string `json:"sid"` // links JWT to Redis session
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
type TokenService struct {
	cfg        *TokenConfig
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewTokenService generates a new Ed25519 server signing key using crypto/rand.
func NewTokenService(cfg *TokenConfig) (*TokenService, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &TokenService{cfg: cfg, privateKey: priv, publicKey: pub}, nil
}

// Issue creates and signs a JWT. SessionID is embedded in the sid claim.
func (s *TokenService) Issue(claims *Claims, sessionID string) (string, error) {
	now := time.Now()
	claims.IssuedAt  = now.Unix()
	claims.ExpiresAt = now.Add(s.cfg.TTL).Unix()
	claims.Issuer    = s.cfg.Issuer
	claims.SessionID = sessionID

	header  := jwtBase64(mustJSONMarshal(map[string]string{"alg": "EdDSA", "typ": "JWT"}))
	payload := jwtBase64(mustJSONMarshal(claims))
	signingInput := header + "." + payload
	sig := ed25519.Sign(s.privateKey, []byte(signingInput))
	return signingInput + "." + jwtBase64(sig), nil
}

// Verify parses and validates a JWT. Signature verified before claims decoded.
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

	if !ed25519.Verify(s.publicKey, []byte(signingInput), sig) {
		return nil, ErrTokenInvalid
	}

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

func jwtBase64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mustJSONMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
