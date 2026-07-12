package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/Harshmaury/verion/internal/identity"
	"github.com/Harshmaury/verion/internal/store"
)

const challengeTTL = 5 * time.Minute

// ErrCloneDetected is returned when the WebAuthn sign count indicates
// the authenticator may have been cloned.
// This is a high-severity security event.
var ErrCloneDetected = errors.New("auth: authenticator clone detected — sign count regression")

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

// ── Registration ──────────────────────────────────────────────────────────────

// BeginRegistration starts the WebAuthn registration ceremony.
func (s *WebAuthnService) BeginRegistration(
	ctx context.Context,
	tenantID string,
	identityID string,
) (*protocol.CredentialCreation, string, error) {
	id, err := s.identitySvc.GetIdentity(ctx, tenantID, identityID)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin registration: %w", err)
	}
	if !id.IsActive() {
		return nil, "", identity.ErrIdentityInactive
	}

	existingCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, identityID, nil)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: list existing credentials: %w", err)
	}
	user := newVerionUser(id, existingCreds)

	creation, session, err := s.wauthn.BeginRegistration(user)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin registration: %w", err)
	}

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: marshal session: %w", err)
	}

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
	id, err := s.identitySvc.GetIdentity(ctx, tenantID, identityID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: load identity: %w", err)
	}

	existingCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, identityID, nil)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: list credentials: %w", err)
	}
	user := newVerionUser(id, existingCreds)

	sessionBytes, err := s.store.GetChallenge(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: get challenge: %w", err)
	}

	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return nil, fmt.Errorf("webauthn: finish: unmarshal session: %w", err)
	}

	credential, err := s.wauthn.FinishRegistration(user, session, r)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: verification failed: %w", err)
	}

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

	createdKey, err := s.repos.Keys.Create(ctx, keyRecord)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create key record: %w", err)
	}

	if id.PrimaryKeyID == nil {
		if err := s.repos.Identities.SetPrimaryKey(ctx, tenantID, identityID, createdKey.ID); err != nil {
			return nil, fmt.Errorf("webauthn: finish: set primary key: %w", err)
		}
	}

	aaguid := fmt.Sprintf("%x", credential.Authenticator.AAGUID)
	credName := "Passkey"
	cred := &identity.Credential{
		IdentityID:   identityID,
		TenantID:     tenantID,
		KeyID:        &createdKey.ID,
		Type:         identity.CredentialTypePasskey,
		Status:       identity.CredentialStatusActive,
		Data:         pubKeyBytes, // COSE public key for assertion verification
		CredentialID: credential.ID,
		SignCount:    int64(credential.Authenticator.SignCount),
		AAGUID:       &aaguid,
		Name:         &credName,
	}

	createdCred, err := s.repos.Credentials.Create(ctx, cred)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create credential: %w", err)
	}

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

	return createdCred, nil
}

// ── Assertion ─────────────────────────────────────────────────────────────────

// BeginAssertion starts the WebAuthn assertion (login) ceremony.
func (s *WebAuthnService) BeginAssertion(
	ctx context.Context,
	tenantID string,
	handle string,
) (*protocol.CredentialAssertion, string, error) {
	// Step 1 — Load identity by handle.
	id, err := s.identitySvc.GetIdentityByHandle(ctx, tenantID, handle)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: %w", err)
	}

	// Step 2 — Verify active.
	if !id.IsActive() {
		return nil, "", identity.ErrIdentityInactive
	}

	// Step 3 — Load active passkey credentials.
	activeStatus := identity.CredentialStatusActive
	allCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, id.ID, &activeStatus)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: list credentials: %w", err)
	}

	// Filter to passkey and hardware token types only.
	var passkeyCreds []*identity.Credential
	for _, c := range allCreds {
		if c.Type == identity.CredentialTypePasskey || c.Type == identity.CredentialTypeHardwareToken {
			passkeyCreds = append(passkeyCreds, c)
		}
	}

	// Step 4 — Build user adapter with credentials.
	user := newVerionUserWithCreds(id, passkeyCreds)

	// Step 5 — Begin login.
	var assertion *protocol.CredentialAssertion
	var session *webauthn.SessionData

	if len(passkeyCreds) == 0 {
		assertion, session, err = s.wauthn.BeginDiscoverableLogin()
	} else {
		assertion, session, err = s.wauthn.BeginLogin(user)
	}
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: %w", err)
	}

	// Step 6 — Marshal session data.
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: marshal session: %w", err)
	}

	// Step 7 — Generate session ID and store in Redis.
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: generate session id: %w", err)
	}
	if err := s.store.SetChallenge(ctx, sessionID, sessionBytes, challengeTTL); err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: store challenge: %w", err)
	}

	// Step 8 — Return assertion options and session ID.
	return assertion, sessionID, nil
}

// FinishAssertion completes the WebAuthn assertion ceremony.
func (s *WebAuthnService) FinishAssertion(
	ctx context.Context,
	tenantID string,
	handle string,
	sessionID string,
	r *http.Request,
) (*identity.Identity, error) {
	// Step 1 — Load identity by handle.
	id, err := s.identitySvc.GetIdentityByHandle(ctx, tenantID, handle)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: %w", err)
	}

	// Step 2 — Verify active.
	if !id.IsActive() {
		return nil, identity.ErrIdentityInactive
	}

	// Step 3 — Load active WebAuthn credentials.
	activeStatus := identity.CredentialStatusActive
	allCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, id.ID, &activeStatus)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: list credentials: %w", err)
	}
	var passkeyCreds []*identity.Credential
	for _, c := range allCreds {
		if c.Type == identity.CredentialTypePasskey || c.Type == identity.CredentialTypeHardwareToken {
			passkeyCreds = append(passkeyCreds, c)
		}
	}

	// Step 4 — Build user adapter.
	user := newVerionUserWithCreds(id, passkeyCreds)

	// Step 5 — Retrieve and consume session (GETDEL — single use).
	sessionBytes, err := s.store.GetChallenge(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: get challenge: %w", err)
	}

	// Step 6 — Unmarshal session data.
	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: unmarshal session: %w", err)
	}

	// Step 7 — Verify signature, challenge, origin.
	credential, err := s.wauthn.FinishLogin(user, session, r)
	if err != nil {
		// Write auth failure audit event.
		actor := id.ID
		_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
			TenantID:   tenantID,
			EventType:  identity.AuditEventAuthFailure,
			EntityType: "identity",
			EntityID:   id.ID,
			ActorID:    &actor,
			Success:    false,
		})
		return nil, fmt.Errorf("webauthn: finish assertion: verification failed: %w", err)
	}

	// Step 8 — Sign count check (clone detection).
	// Find the matching stored credential by credential ID.
	var storedCred *identity.Credential
	for _, c := range passkeyCreds {
		if string(c.CredentialID) == string(credential.ID) {
			storedCred = c
			break
		}
	}

	if storedCred != nil {
		newCount := credential.Authenticator.SignCount
		storedCount := uint32(storedCred.SignCount)
		// Skip check if authenticator doesn't support counters (returns 0).
		if newCount != 0 && newCount <= storedCount {
			actor := id.ID
			_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
				TenantID:   tenantID,
				EventType:  identity.AuditEventAuthFailure,
				EntityType: "identity",
				EntityID:   id.ID,
				ActorID:    &actor,
				Success:    false,
			})
			return nil, ErrCloneDetected
		}

		// Step 9 — Update sign count.
		if err := s.repos.Credentials.UpdateSignCount(ctx, tenantID, storedCred.ID, int64(newCount)); err != nil {
			return nil, fmt.Errorf("webauthn: finish assertion: update sign count: %w", err)
		}

		// Step 10 — Update last used.
		if err := s.repos.Credentials.UpdateLastUsed(ctx, tenantID, storedCred.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("webauthn: finish assertion: update last used: %w", err)
		}
	}

	// Step 11 — Write auth success audit event.
	actor := id.ID
	if err := s.repos.Audit.Insert(ctx, &identity.AuditEvent{
		TenantID:   tenantID,
		EventType:  identity.AuditEventAuthSuccess,
		EntityType: "identity",
		EntityID:   id.ID,
		ActorID:    &actor,
		Success:    true,
	}); err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: write audit event: %w", err)
	}

	// Step 12 — Return authenticated identity.
	return id, nil
}

// ── WebAuthn User adapter ─────────────────────────────────────────────────────

type verionUser struct {
	id          *identity.Identity
	credentials []webauthn.Credential
}

// newVerionUser builds a user adapter with credential IDs only (for registration exclusion list).
func newVerionUser(id *identity.Identity, existingCreds []*identity.Credential) *verionUser {
	waCreds := make([]webauthn.Credential, 0, len(existingCreds))
	for _, c := range existingCreds {
		waCreds = append(waCreds, webauthn.Credential{
			ID: c.CredentialID,
		})
	}
	return &verionUser{id: id, credentials: waCreds}
}

// newVerionUserWithCreds builds a user adapter with full credential data (for assertion).
func newVerionUserWithCreds(id *identity.Identity, creds []*identity.Credential) *verionUser {
	waCreds := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		waCreds = append(waCreds, domainCredToWebAuthn(c))
	}
	return &verionUser{id: id, credentials: waCreds}
}

// domainCredToWebAuthn converts a Verion Credential to webauthn.Credential.
// The credential's Data field contains the COSE-encoded public key
// stored during FinishRegistration.
func domainCredToWebAuthn(c *identity.Credential) webauthn.Credential {
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.Data, // COSE public key stored in Data field
		AttestationType: "none",
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte{},
			SignCount: uint32(c.SignCount),
		},
	}
}

func (u *verionUser) WebAuthnID() []byte                         { return []byte(u.id.ID) }
func (u *verionUser) WebAuthnName() string                       { return u.id.Handle }
func (u *verionUser) WebAuthnDisplayName() string                { return u.id.DisplayName }
func (u *verionUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
func (u *verionUser) WebAuthnIcon() string                       { return "" }

// ── Helpers ───────────────────────────────────────────────────────────────────

func generateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
