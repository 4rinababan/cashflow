package reports

import (
	"encoding/json"
	"errors"
	"net/http"
)

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func errorBody(message string) map[string]string {
	return map[string]string{"error": message}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRange), errors.Is(err, ErrInvalidMonths):
		respondJSON(w, http.StatusBadRequest, errorBody(err.Error()))
	default:
		respondJSON(w, http.StatusInternalServerError, errorBody("internal error"))
	}
}
