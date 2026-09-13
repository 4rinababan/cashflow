package receipts

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
	case errors.Is(err, ErrRateLimited):
		respondJSON(w, http.StatusTooManyRequests, errorBody(err.Error()))
	case errors.Is(err, ErrUnsupportedImage), errors.Is(err, ErrImageTooLarge):
		respondJSON(w, http.StatusBadRequest, errorBody(err.Error()))
	case errors.Is(err, ErrNotConfigured):
		respondJSON(w, http.StatusServiceUnavailable, errorBody(err.Error()))
	case errors.Is(err, ErrScanFailed):
		respondJSON(w, http.StatusUnprocessableEntity, errorBody(err.Error()))
	default:
		respondJSON(w, http.StatusInternalServerError, errorBody("internal error"))
	}
}
