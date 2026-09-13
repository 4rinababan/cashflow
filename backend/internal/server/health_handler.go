package server

import (
	"context"
	"encoding/json"
	"net/http"
)

// Pinger is satisfied by *pgxpool.Pool. Declared narrowly here so the
// readiness handler can be unit-tested without a real database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// healthHandler answers "is the process alive" — it never touches the
// database, so a slow/down Postgres does not make the container look dead to
// an orchestrator's liveness probe.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyHandler answers "can this instance serve real requests" — it checks the
// database, so a load balancer / orchestrator can hold traffic back until the
// database is reachable.
func readyHandler(pinger Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pinger.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
