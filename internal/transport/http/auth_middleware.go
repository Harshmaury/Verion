package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/Harshmaury/verion/internal/auth"
)

// claimsKey is the context key for storing authenticated claims.
type claimsKey struct{}

// RequireAuth is middleware that validates the Bearer token.
// On success: stores *auth.Claims in the request context.
// On failure: returns 401 Unauthorized immediately.
// Does NOT touch the database — pure in-memory JWT verification.
func RequireAuth(tokenSvc *auth.TokenService) func(http.Handler) http.Handler {
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

			// Store verified claims in context for downstream handlers.
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
