package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// maxRequestBodyBytes bounds every request body server-wide, a basic Phase 7
// hardening measure against a client (malicious or just broken) sending an
// unbounded body. Set above receipts/scan's own tighter, image-specific cap
// (internal/receipts.maxImageBytes) so it never interferes with a legitimate
// upload — it only needs to catch requests nothing else already limits.
const maxRequestBodyBytes = 9 << 20 // 9 MiB

// withMiddleware applies the standard chain to every request: recover from a
// panic, tag the request with an id, cap its body size, then log it and
// record metrics. Order matters — recovery must wrap everything else so a
// panic anywhere still gets a clean response.
func withMiddleware(next http.Handler) http.Handler {
	return withRecovery(withRequestID(withMetrics(withBodySizeLimit(withLogging(next)))))
}

func withBodySizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", r.Context().Value(requestIDKey),
		)
	})
}

// withRecovery turns a panic in any handler into a logged 500 instead of a
// crashed connection — a public API must never take the whole process down
// because one request hit a bug.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func newRequestID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
