package http

import (
	"encoding/json"
	"net/http"

	"github.com/Harshmaury/verion/internal/auth"
)

// WebAuthnHandler handles HTTP requests for WebAuthn registration.
type WebAuthnHandler struct {
	svc *auth.WebAuthnService
}

// NewWebAuthnHandler constructs a WebAuthnHandler.
func NewWebAuthnHandler(svc *auth.WebAuthnService) *WebAuthnHandler {
	return &WebAuthnHandler{svc: svc}
}

// RegisterBegin handles POST /v1/auth/register
// Request body: { "tenant_id": "...", "identity_id": "..." }
// Response: WebAuthn CredentialCreationOptions JSON
// Response header: X-Session-ID — must be included in RegisterComplete
func (h *WebAuthnHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		IdentityID string `json:"identity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TenantID == "" || req.IdentityID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id and identity_id are required")
		return
	}

	options, sessionID, err := h.svc.BeginRegistration(r.Context(), req.TenantID, req.IdentityID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("X-Session-ID", sessionID)
	writeJSON(w, http.StatusOK, options)
}

// RegisterComplete handles POST /v1/auth/register/complete
// Required headers: X-Tenant-ID, X-Identity-ID, X-Session-ID
// Request body: WebAuthn AuthenticatorAttestationResponse
// Response: { "credential_id": "...", "type": "passkey", "created_at": "..." }
func (h *WebAuthnHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	tenantID   := r.Header.Get("X-Tenant-ID")
	identityID := r.Header.Get("X-Identity-ID")
	sessionID  := r.Header.Get("X-Session-ID")

	if tenantID == "" || identityID == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID, X-Identity-ID, and X-Session-ID headers are required")
		return
	}

	cred, err := h.svc.FinishRegistration(r.Context(), tenantID, identityID, sessionID, r)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"credential_id": cred.ID,
		"type":          string(cred.Type),
		"created_at":    cred.CreatedAt,
	})
}
