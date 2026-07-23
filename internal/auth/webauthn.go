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

var ErrCloneDetected = errors.New("auth: authenticator clone detected — sign count regression")

type WebAuthnConfig struct {
	RPID          string
	RPDisplayName string
	RPOrigins     []string
}

type WebAuthnService struct {
	wauthn      *webauthn.WebAuthn
	store       *store.RedisStore
	identitySvc identity.IdentityService
	keySvc      identity.KeyService
	repos       *identity.Repositories
}

// New creates a WebAuthnService. TokenService and SessionStore are now
// passed per-call by the handlers (cleaner separation of concerns).
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

func (s *WebAuthnService) BeginRegistration(
	ctx context.Context, tenantID, identityID string,
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
		return nil, "", fmt.Errorf("webauthn: list credentials: %w", err)
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
	sessionID, err := generateChallengeID()
	if err != nil {
		return nil, "", err
	}
	if err := s.store.SetChallenge(ctx, sessionID, sessionBytes, challengeTTL); err != nil {
		return nil, "", fmt.Errorf("webauthn: store challenge: %w", err)
	}
	return creation, sessionID, nil
}

// FinishRegistrationResult holds result of completed registration.
type FinishRegistrationResult struct {
	Credential *identity.Credential
	IdentityID string
	Handle     string
	Type       string
}

func (s *WebAuthnService) FinishRegistration(
	ctx context.Context, tenantID, identityID, sessionID string, r *http.Request,
) (*FinishRegistrationResult, error) {
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
		TenantID: tenantID, IdentityID: identityID,
		KeyType: identity.KeyTypeECDSAP256, Purpose: identity.KeyPurposeAuthentication,
		Algorithm: "ES256", PublicKey: pubKeyBytes, Fingerprint: fingerprint,
		Status: identity.KeyStatusActive,
	}
	createdKey, err := s.repos.Keys.Create(ctx, keyRecord)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create key: %w", err)
	}

	if id.PrimaryKeyID == nil {
		if err := s.repos.Identities.SetPrimaryKey(ctx, tenantID, identityID, createdKey.ID); err != nil {
			return nil, fmt.Errorf("webauthn: finish: set primary key: %w", err)
		}
	}

	aaguid := fmt.Sprintf("%x", credential.Authenticator.AAGUID)
	credName := "Passkey"
	cred := &identity.Credential{
		IdentityID: identityID, TenantID: tenantID, KeyID: &createdKey.ID,
		Type: identity.CredentialTypePasskey, Status: identity.CredentialStatusActive,
		Data: pubKeyBytes, CredentialID: credential.ID,
		SignCount: int64(credential.Authenticator.SignCount),
		AAGUID: &aaguid, Name: &credName,
	}
	createdCred, err := s.repos.Credentials.Create(ctx, cred)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish: create credential: %w", err)
	}

	actor := identityID
	_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
		TenantID: tenantID, EventType: identity.AuditEventCredentialCreated,
		EntityType: "credential", EntityID: createdCred.ID, ActorID: &actor, Success: true,
	})

	return &FinishRegistrationResult{
		Credential: createdCred,
		IdentityID: identityID,
		Handle:     id.Handle,
		Type:       string(id.Type),
	}, nil
}

// ── Assertion ─────────────────────────────────────────────────────────────────

func (s *WebAuthnService) BeginAssertion(
	ctx context.Context, tenantID, handle string,
) (*protocol.CredentialAssertion, string, error) {
	id, err := s.identitySvc.GetIdentityByHandle(ctx, tenantID, handle)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: %w", err)
	}
	if !id.IsActive() {
		return nil, "", identity.ErrIdentityInactive
	}

	activeStatus := identity.CredentialStatusActive
	allCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, id.ID, &activeStatus)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: list: %w", err)
	}
	var passkeyCreds []*identity.Credential
	for _, c := range allCreds {
		if c.Type == identity.CredentialTypePasskey || c.Type == identity.CredentialTypeHardwareToken {
			passkeyCreds = append(passkeyCreds, c)
		}
	}
	user := newVerionUserWithCreds(id, passkeyCreds)

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

	sessionBytes, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: marshal: %w", err)
	}
	sessionID, err := generateChallengeID()
	if err != nil {
		return nil, "", err
	}
	if err := s.store.SetChallenge(ctx, sessionID, sessionBytes, challengeTTL); err != nil {
		return nil, "", fmt.Errorf("webauthn: begin assertion: store: %w", err)
	}
	return assertion, sessionID, nil
}

// FinishAssertionResult holds result of completed assertion.
type FinishAssertionResult struct {
	Identity *identity.Identity
}

func (s *WebAuthnService) FinishAssertion(
	ctx context.Context, tenantID, handle, sessionID string, r *http.Request,
) (*FinishAssertionResult, error) {
	id, err := s.identitySvc.GetIdentityByHandle(ctx, tenantID, handle)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: %w", err)
	}
	if !id.IsActive() {
		return nil, identity.ErrIdentityInactive
	}

	activeStatus := identity.CredentialStatusActive
	allCreds, err := s.repos.Credentials.ListByIdentity(ctx, tenantID, id.ID, &activeStatus)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: list: %w", err)
	}
	var passkeyCreds []*identity.Credential
	for _, c := range allCreds {
		if c.Type == identity.CredentialTypePasskey || c.Type == identity.CredentialTypeHardwareToken {
			passkeyCreds = append(passkeyCreds, c)
		}
	}
	user := newVerionUserWithCreds(id, passkeyCreds)

	sessionBytes, err := s.store.GetChallenge(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: get challenge: %w", err)
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return nil, fmt.Errorf("webauthn: finish assertion: unmarshal: %w", err)
	}

	credential, err := s.wauthn.FinishLogin(user, session, r)
	if err != nil {
		actor := id.ID
		_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
			TenantID: tenantID, EventType: identity.AuditEventAuthFailure,
			EntityType: "identity", EntityID: id.ID, ActorID: &actor, Success: false,
		})
		return nil, fmt.Errorf("webauthn: finish assertion: verify: %w", err)
	}

	var storedCred *identity.Credential
	for _, c := range passkeyCreds {
		if string(c.CredentialID) == string(credential.ID) {
			storedCred = c; break
		}
	}
	if storedCred != nil {
		newCount := credential.Authenticator.SignCount
		if newCount != 0 && newCount <= uint32(storedCred.SignCount) {
			actor := id.ID
			_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
				TenantID: tenantID, EventType: identity.AuditEventAuthFailure,
				EntityType: "identity", EntityID: id.ID, ActorID: &actor, Success: false,
			})
			return nil, ErrCloneDetected
		}
		_ = s.repos.Credentials.UpdateSignCount(ctx, tenantID, storedCred.ID, int64(newCount))
		_ = s.repos.Credentials.UpdateLastUsed(ctx, tenantID, storedCred.ID, time.Now())
	}

	actor := id.ID
	_ = s.repos.Audit.Insert(ctx, &identity.AuditEvent{
		TenantID: tenantID, EventType: identity.AuditEventAuthSuccess,
		EntityType: "identity", EntityID: id.ID, ActorID: &actor, Success: true,
	})

	return &FinishAssertionResult{Identity: id}, nil
}

// ── User adapter ──────────────────────────────────────────────────────────────

type verionUser struct {
	id          *identity.Identity
	credentials []webauthn.Credential
}

func newVerionUser(id *identity.Identity, creds []*identity.Credential) *verionUser {
	wa := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds { wa = append(wa, webauthn.Credential{ID: c.CredentialID}) }
	return &verionUser{id: id, credentials: wa}
}

func newVerionUserWithCreds(id *identity.Identity, creds []*identity.Credential) *verionUser {
	wa := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds { wa = append(wa, domainCredToWebAuthn(c)) }
	return &verionUser{id: id, credentials: wa}
}

func domainCredToWebAuthn(c *identity.Credential) webauthn.Credential {
	return webauthn.Credential{
		ID: c.CredentialID, PublicKey: c.Data, AttestationType: "none",
		Authenticator: webauthn.Authenticator{AAGUID: []byte{}, SignCount: uint32(c.SignCount)},
	}
}

func (u *verionUser) WebAuthnID() []byte                         { return []byte(u.id.ID) }
func (u *verionUser) WebAuthnName() string                       { return u.id.Handle }
func (u *verionUser) WebAuthnDisplayName() string                { return u.id.DisplayName }
func (u *verionUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
func (u *verionUser) WebAuthnIcon() string                       { return "" }

func generateChallengeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate challenge id: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
