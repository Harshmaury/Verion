package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Harshmaury/verion/internal/auth"
	"github.com/Harshmaury/verion/internal/store"
)

// WebAuthnHandler handles HTTP requests for WebAuthn registration.
type WebAuthnHandler struct {
	svc          *auth.WebAuthnService
	tokenSvc     *auth.TokenService
	sessionStore *store.SessionStore
}

func NewWebAuthnHandler(svc *auth.WebAuthnService, tokenSvc *auth.TokenService, sessionStore *store.SessionStore) *WebAuthnHandler {
	return &WebAuthnHandler{svc: svc, tokenSvc: tokenSvc, sessionStore: sessionStore}
}

func (h *WebAuthnHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		IdentityID string `json:"identity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body"); return
	}
	if req.TenantID == "" || req.IdentityID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and identity_id are required"); return
	}
	options, sessionID, err := h.svc.BeginRegistration(r.Context(), req.TenantID, req.IdentityID)
	if err != nil {
		writeServiceError(w, err); return
	}
	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusOK, options)
}

func (h *WebAuthnHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	tenantID   := r.Header.Get("X-Tenant-ID")
	identityID := r.Header.Get("X-Identity-ID")
	sessionID  := r.Header.Get("X-Session-ID")
	if tenantID == "" || identityID == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID, X-Identity-ID, and X-Session-ID headers required"); return
	}

	result, err := h.svc.FinishRegistration(r.Context(), tenantID, identityID, sessionID, r)
	if err != nil {
		writeServiceError(w, err); return
	}

	// Create session in Redis.
	authSessionID := generateSessionID()
	session := &store.SessionData{
		SessionID:  authSessionID,
		IdentityID: result.IdentityID,
		TenantID:   tenantID,
		Handle:     result.Handle,
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
		DeviceInfo: r.Header.Get("User-Agent"),
		IPAddress:  r.RemoteAddr,
	}
	if err := h.sessionStore.Create(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "session creation failed"); return
	}

	// Issue JWT with session ID embedded.
	token, err := h.tokenSvc.Issue(&auth.Claims{
		Subject:  result.IdentityID,
		TenantID: tenantID,
		Handle:   result.Handle,
		Type:     result.Type,
	}, authSessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issuance failed"); return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"credential_id": result.Credential.ID,
		"type":          string(result.Credential.Type),
		"created_at":    result.Credential.CreatedAt,
		"token":         token,
		"session_id":    authSessionID,
	})
}
