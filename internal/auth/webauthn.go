package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/Harshmaury/verion/internal/identity"
	"github.com/Harshmaury/verion/internal/store"
)

const challengeTTL = 5 * time.Minute

// WebAuthnConfig holds configuration for the WebAuthn relying party.
type WebAuthnConfig struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

// WebAuthnService handles passkey registration and assertion ceremonies.
type WebAuthnService struct {
	wauthn      *webauthn.WebAuthn
	store       *store.RedisStore
	identitySvc identity.IdentityService
	keySvc      identity.KeyService
	repos       *identity.Repositories
}

// New creates a WebAuthnService.
func New(
	cfg WebAuthnConfig,
	redisStore *store.RedisStore,
	identitySvc identity.IdentityService,
	keySvc identity.KeyService,
	repos *identity.Repositories,
) (*WebAuthnService, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:          cfg.RPID,
		RPDisplayName: cfg.RPDisplayName,
		RPOrigins:     cfg.RPOrigins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn: init: %w", err)
	}
	return &WebAuthnService{
		wauthn:      w,
		store:       redisStore,
		identitySvc: identitySvc,
		keySvc:      keySvc,
		repos:       repos,
	}, nil
}

// BeginRegistration starts the WebAuthn registration ceremony.
// Returns CredentialCreationOptions and a session ID to be sent to the client.
func (s *WebAuthnService) BeginRegistration(
	ctx context.Context,
	tenantID string,
	identityID string,
) (*protocol.CredentialCreation, string, error) {
	// Step 1 — Load identity.
	id, err := s.identitySvc.GetIdentity(ctx, tenantID, identityID)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin registration: %w", err)
	}

	// Step 2 — Verify active.
	if !id.IsActive() {
		return nil, "", identity.ErrIdentityInactive
	}

	// Step 3 — Build user adapter with existing credentials for exclusion list.
	existingCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, identityID, nil)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: list existing credentials: %w", err)
	}
	user := newVerionUser(id, existingCreds)

	// Step 4 — Call library BeginRegistration.
	creation, session, err := s.wauthn.BeginRegistration(user)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin registration: %w", err)
	}

	// Step 5 — Marshal session data to JSON.
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: marshal session: %w", err)
	}

	// Step 6 — Generate session ID and store in Redis with 5-minute TTL.
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: generate session id: %w", err)
	}
	if err := s.store.SetChallenge(ctx, sessionID, sessionBytes, challengeTTL); err != nil {
		return nil, "", fmt.Errorf("webauthn: store challenge: %w", err)
	}

	return creation, sessionID, nil
}

// FinishRegistration completes the WebAuthn registration ceremony.
func (s *WebAuthnService) FinishRegistration(
	ctx context.Context,
	tenantID string,
	identityID string,
	sessionID string,
	r *http.Request,
) (*identity.Credential, error) {
	// Step 1 — Load identity.
	id, err := s.identitySvc.GetIdentity(ctx, tenantID, identityID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: load identity: %w", err)
	}

	// Step 2 — Build user adapter.
	existingCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, identityID, nil)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: list credentials: %w", err)
	}
	user := newVerionUser(id, existingCreds)

	// Step 3 — Retrieve and atomically delete session (GETDEL — single use).
	sessionBytes, err := s.store.GetChallenge(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: get challenge: %w", err)
	}

	// Step 4 — Unmarshal session data.
	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return nil, fmt.Errorf("webauthn: finish: unmarshal session: %w", err)
	}

	// Step 5 — Verify: challenge matches, signature valid, origin correct.
	credential, err := s.wauthn.FinishRegistration(user, session, r)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: verification failed: %w", err)
	}

	// Steps 6-9 — Extract public key, compute fingerprint.
	pubKeyBytes := credential.PublicKey
	sum := sha256.Sum256(pubKeyBytes)
	fingerprint := fmt.Sprintf("%x", sum)

	keyRecord := &identity.IdentityKey{
		TenantID:    tenantID,
		IdentityID:  identityID,
		KeyType:     identity.KeyTypeECDSAP256,
		Purpose:     identity.KeyPurposeAuthentication,
		Algorithm:   "ES256",
		PublicKey:   pubKeyBytes,
		Fingerprint: fingerprint,
		Status:      identity.KeyStatusActive,
	}

	// Step 10 — Persist public key record.
	createdKey, err := s.repos.Keys.Create(ctx, keyRecord)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create key record: %w", err)
	}

	// Step 11 — Set primary key if identity has none.
	if id.PrimaryKeyID == nil {
		if err := s.repos.Identities.SetPrimaryKey(ctx, tenantID, identityID, createdKey.ID); err != nil {
			return nil, fmt.Errorf("webauthn: finish: set primary key: %w", err)
		}
	}

	// Step 12 — Build and store credential record.
	aaguid := fmt.Sprintf("%x", credential.Authenticator.AAGUID)
	credName := "Passkey"
	cred := &identity.Credential{
		IdentityID:   identityID,
		TenantID:     tenantID,
		KeyID:        &createdKey.ID,
		Type:         identity.CredentialTypePasskey,
		Status:       identity.CredentialStatusActive,
		CredentialID: credential.ID,
		SignCount:    int64(credential.Authenticator.SignCount),
		AAGUID:       &aaguid,
		Name:         &credName,
	}

	createdCred, err := s.repos.Credentials.Create(ctx, cred)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create credential: %w", err)
	}

	// Step 13 — Write audit event.
	actor := identityID
	if err := s.repos.Audit.Insert(ctx, &identity.AuditEvent{
		TenantID:   tenantID,
		EventType:  identity.AuditEventCredentialCreated,
		EntityType: "credential",
		EntityID:   createdCred.ID,
		ActorID:    &actor,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("webauthn: finish: write audit event: %w", err)
	}

	// Step 14 — Return stored credential.
	return createdCred, nil
}

// ── WebAuthn User adapter ─────────────────────────────────────────────────────

type verionUser struct {
	id          *identity.Identity
	credentials []webauthn.Credential
}

func newVerionUser(id *identity.Identity, existingCreds []*identity.Credential) *verionUser {
	waCreds := make([]webauthn.Credential, 0, len(existingCreds))
	for _, c := range existingCreds {
		// Populate credential ID for exclusion list during registration.
		waCreds = append(waCreds, webauthn.Credential{
			ID: c.CredentialID,
		})
	}
	return &verionUser{id: id, credentials: waCreds}
}

func (u *verionUser) WebAuthnID() []byte                        { return []byte(u.id.ID) }
func (u *verionUser) WebAuthnName() string                      { return u.id.Handle }
func (u *verionUser) WebAuthnDisplayName() string               { return u.id.DisplayName }
func (u *verionUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
func (u *verionUser) WebAuthnIcon() string                      { return "" } // deprecated in spec

// ── Helpers ───────────────────────────────────────────────────────────────────

// generateSessionID returns a 32-char hex string from 16 random bytes.
// Uses crypto/rand — not sequential.
func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
