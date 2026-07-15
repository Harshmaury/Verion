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
// Returns credential info plus JWT token.
func (h *WebAuthnHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	tenantID  := r.Header.Get("X-Tenant-ID")
	identityID := r.Header.Get("X-Identity-ID")
	sessionID := r.Header.Get("X-Session-ID")

	if tenantID == "" || identityID == "" || sessionID == "" {
		writeError(w, http.StatusBadRequest, "X-Tenant-ID, X-Identity-ID, and X-Session-ID headers are required")
		return
	}

	result, err := h.svc.FinishRegistration(r.Context(), tenantID, identityID, sessionID, r)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"credential_id": result.Credential.ID,
		"type":          string(result.Credential.Type),
		"created_at":    result.Credential.CreatedAt,
		"token":         result.Token,
	})
}
