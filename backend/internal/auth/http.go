package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"cashflow-backend/internal/users"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		respondJSON(w, http.StatusBadRequest, errorBody("invalid request body"))
		return false
	}
	return true
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func errorBody(message string) map[string]string {
	return map[string]string{"error": message}
}

// writeServiceError maps a Service error to the right HTTP status. Unknown
// errors become a generic 500 — never leak internals to the client.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		respondJSON(w, http.StatusUnauthorized, errorBody("invalid email or password"))
	case errors.Is(err, ErrInvalidToken):
		respondJSON(w, http.StatusUnauthorized, errorBody("invalid or expired token"))
	case errors.Is(err, ErrPasswordTooShort):
		respondJSON(w, http.StatusBadRequest, errorBody("password must be at least 8 characters"))
	case errors.Is(err, ErrDisplayNameRequired):
		respondJSON(w, http.StatusBadRequest, errorBody("displayName is required"))
	case errors.Is(err, users.ErrInvalidEmail):
		respondJSON(w, http.StatusBadRequest, errorBody("invalid email address"))
	case errors.Is(err, users.ErrEmailTaken):
		respondJSON(w, http.StatusConflict, errorBody("email already registered"))
	case errors.Is(err, users.ErrNotFound):
		respondJSON(w, http.StatusNotFound, errorBody("user not found"))
	default:
		respondJSON(w, http.StatusInternalServerError, errorBody("internal error"))
	}
}
