package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Harshmaury/verion/internal/auth"
	"github.com/Harshmaury/verion/internal/identity"
	"github.com/Harshmaury/verion/internal/store"
)

// Gateway is the HTTP server exposing Verion's REST API.
type Gateway struct {
	server         *http.Server
	identitySvc    identity.IdentityService
	tenantSvc      identity.TenantService
	keySvc         identity.KeyService
	tokenSvc       *auth.TokenService
	sessionStore   *store.SessionStore
	wauthn         *WebAuthnHandler
	login          *LoginHandler
	sessionHandler *SessionHandler
}

// New creates a Gateway, registers all routes, applies middleware stack.
func New(
	addr           string,
	identitySvc    identity.IdentityService,
	tenantSvc      identity.TenantService,
	keySvc         identity.KeyService,
	tokenSvc       *auth.TokenService,
	sessionStore   *store.SessionStore,
	wauthn         *WebAuthnHandler,
	login          *LoginHandler,
	sessionHandler *SessionHandler,
) *Gateway {
	g := &Gateway{
		identitySvc:    identitySvc,
		tenantSvc:      tenantSvc,
		keySvc:         keySvc,
		tokenSvc:       tokenSvc,
		sessionStore:   sessionStore,
		wauthn:         wauthn,
		login:          login,
		sessionHandler: sessionHandler,
	}

	mux := http.NewServeMux()
	g.registerRoutes(mux)

	handler := Chain(mux, Recover, RequestID, Logger, CORS)
	g.server = &http.Server{Addr: addr, Handler: handler}
	return g
}

func (g *Gateway) Start() error              { return g.server.ListenAndServe() }
func (g *Gateway) Shutdown(ctx context.Context) error { return g.server.Shutdown(ctx) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode failed", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrNotFound),
		errors.Is(err, identity.ErrTenantNotFound),
		errors.Is(err, identity.ErrKeyNotFound),
		errors.Is(err, identity.ErrCredentialNotFound),
		errors.Is(err, identity.ErrRecoveryNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, identity.ErrAlreadyExists),
		errors.Is(err, identity.ErrHandleTaken),
		errors.Is(err, identity.ErrVersionConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, identity.ErrInvalidInput),
		errors.Is(err, identity.ErrInvalidHandle):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, identity.ErrIdentityTerminal),
		errors.Is(err, identity.ErrTenantInactive),
		errors.Is(err, identity.ErrIdentityInactive),
		errors.Is(err, identity.ErrKeyCompromised),
		errors.Is(err, identity.ErrKeyNotUsable):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, identity.ErrUnauthorized):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		slog.Error("unhandled service error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
