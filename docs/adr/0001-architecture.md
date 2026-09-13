# ADR 0001 — Stack, layering, and the multi-user data model

## Status

Accepted (Phase 1).

## Context

Cashflow is a personal income/expense tracker that will be published free on the
Play Store, so it must support many independent users on shared infrastructure
from day one, not as a later migration. It also has an AI-assisted feature
(receipt → draft transaction) that must never blur which user owns which data.

## Decision

### Mobile: Flutter

One codebase for Android now, iOS later if wanted. State management: Riverpod
(testable, no `BuildContext`-coupled global state). Navigation: `go_router`
(deep-linkable, works well with Riverpod). UI: Material 3, since that is what
Play Store users expect from a modern Android app and it ships in Flutter
out of the box (no design-system build-out needed to look "professional").

### Backend: Go, stdlib `net/http`

Go compiles to a single static binary, has cheap concurrency for many users,
and a small, well-understood standard library. Go 1.22 added method+path
patterns to `http.ServeMux` (`mux.HandleFunc("POST /transactions", ...)`), which
covers routing needs without adding a framework dependency. Add one only if a
real need appears (e.g. OpenAPI generation) — do not add it pre-emptively.

Layering (`internal/` — not importable outside the module):
- `internal/config` — env-based configuration, no framework.
- `internal/server` — HTTP wiring: router, middleware, handlers call into...
- `internal/platform/db` — Postgres connection pool (`pgx`) + migration runner.
- Later phases add `internal/auth`, `internal/ledger`, `internal/receipts` as
  vertical feature packages, each owning its own handlers + queries. Do not
  introduce a generic repository/service/controller trio ahead of need — Go
  favours small interfaces defined at the point of use.

### Database: PostgreSQL, plain SQL migrations, no ORM

`pgx` (the native Postgres driver) + hand-written SQL + `golang-migrate` for
schema changes. An ORM buys little for this domain's query shapes and costs
control over exactly what SQL runs against user financial data. Migrations are
the schema source of truth, same convention as the trading platforms in this
workspace.

### Ownership model: every row belongs to exactly one user

There is no "tenant" or "organisation" concept here — a cashflow ledger is
personal. Every domain table (accounts, categories, transactions, receipts)
carries a `user_id` column, and **every** query is scoped to
`WHERE user_id = $authenticated_user_id`, enforced in the repository layer, not
left to handlers to remember. This is the same discipline the trading platforms
in this workspace apply via `ITenantOwned` + a global query filter — here it is
simpler (no shared-staff-across-tenants case), so a repository-level scoping
convention is enough; it must still be proven by a "user A cannot read/write
user B's row" test for every table, the same bar the trading platforms hold
`ITenantOwned` entities to.

### Auth: email + password, JWT bearer

Phase 2. Password hashing via bcrypt (or argon2id if the Go toolchain used at
the time makes that easy) — never store or log plaintext. Short-lived access
token + a rotated, revocable refresh token, mirroring the pattern already
proven in `AutoTradeSaaS` / `ForexTradingPlatform` in this workspace. Social
sign-in (Google) is a candidate for Phase 6 polish, not required for v1.

## Consequences

- No multi-tenancy complexity (no tenant switching, no staff-across-tenants) —
  the data model stays simple, which matters because this ships to the public
  with no operations team behind it.
- Go's static binary + Docker makes the backend cheap to host on almost any
  VPS or container platform; the specific host is a Phase 8 decision, not
  baked in now.
- Because this will have real (if small) financial data from strangers once
  published, every phase from 2 onward treats "can user A ever see user B's
  data" as a release-blocking question, not a nice-to-have test.
