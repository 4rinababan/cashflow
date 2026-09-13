package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userIDContextKey contextKey = "auth_user_id"

// RequireAuth verifies the "Authorization: Bearer <token>" header and, on
// success, puts the token's subject (user ID) in the request context for
// downstream handlers/repositories to scope their queries by. Every
// user-owned resource's handlers (Phase 3+) go through this.
func RequireAuth(issuer *TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				respondJSON(w, http.StatusUnauthorized, errorBody("missing bearer token"))
				return
			}

			userID, err := issuer.VerifyAccessToken(token)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, errorBody("invalid or expired token"))
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDContextKey, userID)))
		})
	}
}

// UserIDFromContext returns the authenticated user's ID, set by RequireAuth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDContextKey).(string)
	return id, ok
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(h, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}
