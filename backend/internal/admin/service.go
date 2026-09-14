// Package admin serves an operator-only dashboard (never linked from the
// public API surface, never used by the mobile app): signup counts and
// growth for Cashflow, a best-effort total user count for the sibling
// gawean-management app if a read-only connection to it has been
// configured, and a recent-errors panel fed by scripts/log-watch.py. Gated
// entirely behind HTTP Basic Auth (see basicauth.go) — this is an internal
// ops tool, not a customer-facing feature.
package admin

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service holds everything the dashboard needs to render itself.
type Service struct {
	cashflow *pgxpool.Pool
	// gawean is nil when GAWEAN_DATABASE_URL isn't configured — every
	// method here degrades gracefully (returns a "not connected" result)
	// rather than erroring, since this whole integration is optional.
	gawean *pgxpool.Pool

	errorLogPath string
}

// NewService wires the dashboard to Cashflow's own pool (required) and, if
// gaweanDatabaseURL is non-empty, a second read-only pool for the sibling
// app's total user count. A failed Gawean connection is logged by the
// caller (see cmd/api/main.go) and treated the same as "not configured" —
// it must never prevent Cashflow's own server from starting.
func NewService(cashflowPool *pgxpool.Pool, gaweanPool *pgxpool.Pool, errorLogPath string) *Service {
	return &Service{cashflow: cashflowPool, gawean: gaweanPool, errorLogPath: errorLogPath}
}

// ConnectGawean opens (and pings) a read-only pool to the sibling app's
// database. Returns (nil, nil) when databaseURL is empty — "not configured"
// is not an error. Exported as a standalone helper (rather than folded into
// NewService) so main.go can log a connection failure without treating it
// as fatal to the whole server.
func ConnectGawean(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, nil
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConns = 2 // this is a read-only, low-traffic side connection — never compete with the app's own pool

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
