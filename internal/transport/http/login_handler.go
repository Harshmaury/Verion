package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Harshmaury/verion/internal/auth"
	"github.com/Harshmaury/verion/internal/store"
)

// LoginHandler handles HTTP requests for WebAuthn login (assertion).
type LoginHandler struct {
	svc          *auth.WebAuthnService
	tokenSvc     *auth.TokenService
	sessionStore *store.SessionStore
}

func NewLoginHandler(svc *auth.WebAuthnService, tokenSvc *auth.TokenService, sessionStore *store.SessionStore) *LoginHandler {
	return &LoginHandler{svc: svc, tokenSvc: tokenSvc, sessionStore: sessionStore}
}

func (h *LoginHandler) LoginBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `json:"tenant_id"`
		Handle   string `json:"handle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body"); return
	}
	if req.TenantID == "" || req.Handle == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and handle are required"); return
	}
	options, sessionID, err := h.svc.BeginAssertion(r.Context(), req.TenantID, req.Handle)
	if err != nil {
		writeServiceError(w, err); return
	}
	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusOK, options)
}

func (h *LoginHandler) LoginComplete(w http.ResponseWriter, r *http.Request) {
	tenantID  := r.Header.Get("X-Tenant-ID")
	handle    := r.Header.Get("X-Handle")
	sessionID := r.Header.Get("X-Session-ID")
	if tenantID == "" || handle == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID, X-Handle, and X-Session-ID headers required"); return
	}

	result, err := h.svc.FinishAssertion(r.Context(), tenantID, handle, sessionID, r)
	if err != nil {
		if errors.Is(err, auth.ErrCloneDetected) {
			slog.Error("authenticator clone detected", "tenant_id", tenantID, "handle", handle)
			writeError(w, http.StatusForbidden, "authentication failed"); return
		}
		writeServiceError(w, err); return
	}

	// Create session in Redis.
	authSessionID := generateSessionID()
	session := &store.SessionData{
		SessionID:  authSessionID,
		IdentityID: result.Identity.ID,
		TenantID:   tenantID,
		Handle:     result.Identity.Handle,
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
		Subject:  result.Identity.ID,
		TenantID: result.Identity.TenantID,
		Handle:   result.Identity.Handle,
		Type:     string(result.Identity.Type),
	}, authSessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issuance failed"); return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"identity_id":   result.Identity.ID,
		"tenant_id":     result.Identity.TenantID,
		"handle":        result.Identity.Handle,
		"authenticated": true,
		"token":         token,
		"session_id":    authSessionID,
	})
}
