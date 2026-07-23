package http

import (
	"log/slog"
	"net/http"

	"github.com/Harshmaury/verion/internal/store"
)

// SessionHandler handles HTTP requests for session management.
type SessionHandler struct {
	sessionStore *store.SessionStore
}

// NewSessionHandler constructs a SessionHandler.
func NewSessionHandler(s *store.SessionStore) *SessionHandler {
	return &SessionHandler{sessionStore: s}
}

// Logout handles POST /v1/auth/logout
// Deletes the current session. JWT remains technically valid until expiry
// but RequireAuth will reject it because the session no longer exists.
func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if err := h.sessionStore.Delete(r.Context(), claims.SessionID); err != nil {
		// Log but don't fail — session may already be expired.
		slog.Warn("session delete failed on logout",
			"session_id", claims.SessionID,
			"err", err,
		)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// ListSessions handles GET /v1/auth/sessions
// Returns all active sessions for the authenticated identity.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	sessions, err := h.sessionStore.ListByIdentity(r.Context(), claims.Subject)
	if err != nil {
		slog.Error("list sessions failed", "identity_id", claims.Subject, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// RevokeSession handles DELETE /v1/auth/sessions/{session_id}
// Revokes a specific session. Identity can only revoke their own sessions.
func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	// Verify the session belongs to the requesting identity.
	session, err := h.sessionStore.Get(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.IdentityID != claims.Subject {
		writeError(w, http.StatusForbidden, "cannot revoke another identity's session")
		return
	}

	if err := h.sessionStore.Delete(r.Context(), sessionID); err != nil {
		slog.Error("revoke session failed", "session_id", sessionID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
