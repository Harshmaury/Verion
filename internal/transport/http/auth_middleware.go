package http

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Harshmaury/verion/internal/auth"
	"github.com/Harshmaury/verion/internal/store"
)

type claimsKey struct{}

// RequireAuth validates the Bearer token AND checks Redis session existence.
// Rejects requests if: token missing, JWT invalid/expired, or session deleted.
func RequireAuth(tokenSvc *auth.TokenService, sessionStore *store.SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := tokenSvc.Verify(token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Validate session exists in Redis.
			// If session was deleted (logout), reject even a valid JWT.
			if claims.SessionID != "" {
				if _, err := sessionStore.Get(r.Context(), claims.SessionID); err != nil {
					slog.Debug("session not found", "session_id", claims.SessionID, "err", err)
					writeError(w, http.StatusUnauthorized, "session expired or revoked")
					return
				}
			}

			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext retrieves authenticated claims from the request context.
// Returns nil (never panics) if the request was not authenticated.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(claimsKey{}).(*auth.Claims)
	return claims
}
