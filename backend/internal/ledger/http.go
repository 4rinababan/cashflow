package ledger

import (
	"encoding/json"
	"errors"
	"net/http"
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
	case errors.Is(err, ErrNotFound):
		respondJSON(w, http.StatusNotFound, errorBody("not found"))
	case errors.Is(err, ErrInUse):
		respondJSON(w, http.StatusConflict, errorBody(err.Error()))
	case errors.Is(err, ErrAccountArchived), errors.Is(err, ErrCategoryArchived):
		respondJSON(w, http.StatusConflict, errorBody(err.Error()))
	case errors.Is(err, ErrInvalidName), errors.Is(err, ErrInvalidCurrency),
		errors.Is(err, ErrInvalidDirection), errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrCategoryKindMismatch):
		respondJSON(w, http.StatusBadRequest, errorBody(err.Error()))
	default:
		respondJSON(w, http.StatusInternalServerError, errorBody("internal error"))
	}
}
