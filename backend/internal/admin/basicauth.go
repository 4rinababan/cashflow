package admin

import (
	"crypto/subtle"
	"net/http"
)

// RequireBasicAuth gates every /admin route behind a single operator
// username/password (ADMIN_USER/ADMIN_PASSWORD) — deliberately simple for a
// solo-founder internal tool, not a second user-facing auth system.
// Constant-time comparison avoids a timing side-channel on the credentials.
func RequireBasicAuth(user, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotUser, gotPassword, ok := r.BasicAuth()
			userOK := ok && subtle.ConstantTimeCompare([]byte(gotUser), []byte(user)) == 1
			passOK := ok && subtle.ConstantTimeCompare([]byte(gotPassword), []byte(password)) == 1
			if !userOK || !passOK {
				w.Header().Set("WWW-Authenticate", `Basic realm="Cashflow admin"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
