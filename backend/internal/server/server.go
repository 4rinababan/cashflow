// Package server wires HTTP routing, middleware, and lifecycle for the
// Cashflow API.
package server

import (
	"context"
	"net/http"
	"time"
)

// Server wraps an *http.Server with the app's router and sane timeouts, and
// exposes just what main needs: start, and a graceful shutdown.
type Server struct {
	httpServer *http.Server
}

// New builds a Server listening on port, wired to deps.
func New(port string, deps Deps) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:              ":" + port,
			Handler:           NewRouter(deps),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			// WriteTimeout covers the whole request, including handler work —
			// not just 15s: live-testing Phase 4's AI receipt scanning showed
			// a real reasoning-model round trip can take longer than that on
			// its own, and a too-short WriteTimeout silently kills the
			// connection before the handler can respond (curl saw "empty
			// reply from server", not a clean error). 60s gives it headroom
			// over the AI HTTP client's own 30s timeout (internal/receipts.NewOpenAIScanner).
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// ListenAndServe blocks until the server stops; returns http.ErrServerClosed
// on a clean Shutdown.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown stops accepting new connections and waits for in-flight requests
// to finish, or ctx to expire, whichever comes first.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
