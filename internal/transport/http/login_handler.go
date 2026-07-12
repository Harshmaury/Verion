package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Harshmaury/verion/internal/auth"
)

// LoginHandler handles HTTP requests for WebAuthn login (assertion).
type LoginHandler struct {
	svc *auth.WebAuthnService
}

// NewLoginHandler constructs a LoginHandler.
func NewLoginHandler(svc *auth.WebAuthnService) *LoginHandler {
	return &LoginHandler{svc: svc}
}

// LoginBegin handles POST /v1/auth/login
// Request body: { "tenant_id": "...", "handle": "..." }
// Response: WebAuthn CredentialRequestOptions JSON
// Response header: X-Session-ID
func (h *LoginHandler) LoginBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID string `json:"tenant_id"`
		Handle   string `json:"handle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TenantID == "" || req.Handle == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and handle are required")
		return
	}

	options, sessionID, err := h.svc.BeginAssertion(r.Context(), req.TenantID, req.Handle)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusOK, options)
}

// LoginComplete handles POST /v1/auth/login/complete
// Required headers: X-Tenant-ID, X-Handle, X-Session-ID
// Request body: WebAuthn AuthenticatorAssertionResponse
// Response: { "identity_id": "...", "tenant_id": "...", "handle": "...", "authenticated": true }
func (h *LoginHandler) LoginComplete(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	handle   := r.Header.Get("X-Handle")
	sessionID := r.Header.Get("X-Session-ID")

	if tenantID == "" || handle == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID, X-Handle, and X-Session-ID headers are required")
		return
	}

	id, err := h.svc.FinishAssertion(r.Context(), tenantID, handle, sessionID, r)
	if err != nil {
		if errors.Is(err, auth.ErrCloneDetected) {
			slog.Error("authenticator clone detected",
				"tenant_id", tenantID,
				"handle", handle,
			)
			writeError(w, http.StatusForbidden, "authentication failed")
			return
		}
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"identity_id":   id.ID,
		"tenant_id":     id.TenantID,
		"handle":        id.Handle,
		"authenticated": true,
	})
}
