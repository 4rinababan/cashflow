# Cashflow — backend (Go)

## Layout

```
cmd/api/            entrypoint: loads config, connects Postgres, migrates, serves
internal/config/    env-based configuration + validation
internal/server/    router, middleware (recovery/logging/request-id), health/ready
internal/platform/db/  Postgres pool + dependency-free migration runner
internal/users/     User record + Postgres store (email normalization; no auth logic)
internal/auth/       Registration/login/refresh/logout, bcrypt hashing, JWT access
                      tokens, hashed+rotated opaque refresh tokens, RequireAuth middleware
internal/ledger/     Accounts, categories, transactions: CRUD + ownership rules,
                      running balance, category-kind/direction matching, soft-delete
internal/receipts/   AI receipt scanning: chat-completions call -> draft (never
                      saves a transaction itself), per-user daily rate limit
internal/reports/    Read-only aggregates over the ledger: monthly summary,
                      category breakdown, month-over-month trend
migrations/          *.up.sql applied in filename order; *.down.sql kept for a
                      human to run manually if a rollback is ever needed — the
                      runner does not execute them automatically
```

## Auth (Phase 2)

```
POST /auth/register   {email, password, displayName} -> 201 {user, accessToken, accessTokenExpiresAt, refreshToken}
POST /auth/login      {email, password}               -> 200 same shape
POST /auth/refresh    {refreshToken}                  -> 200 same shape; rotates the token (the old one stops working)
POST /auth/logout     {refreshToken}, needs Authorization: Bearer <accessToken> -> 204
GET  /auth/me         Authorization: Bearer <accessToken>                      -> 200 {id, email, displayName}
DELETE /auth/me       Authorization: Bearer <accessToken>                      -> 204 (Phase 8: permanently deletes the
                                                                                        account and everything it owns)
```

- Access tokens are JWT (HS256, 15 min TTL). Refresh tokens are random opaque
  strings (30 day TTL); only their sha256 hash is stored, and each refresh
  revokes the old one and issues a new one (rotation) — reusing an
  already-rotated token is rejected.
- `JWT_SIGNING_KEY` may be blank in `ENV=development` (an ephemeral key is
  generated at startup — logged as a warning, tokens won't survive a restart);
  it is **required** otherwise (`config.Load` fails fast if missing).
- Login and registration failures return the same generic error
  (`invalid email or password` / wrong password vs. unknown email are
  indistinguishable) so the API can't be used to enumerate registered emails.
- On successful registration, `internal/auth.Service.afterRegister` (set in
  `cmd/api/main.go`) calls into `internal/ledger` to seed the new user's 6
  default categories — auth never imports ledger directly, keeping the
  dependency one-way.
- `DELETE /auth/me` (Phase 8) deletes the `users` row; every other table
  (`accounts`, `categories`, `transactions`, `refresh_tokens`,
  `receipt_scans`) has an `ON DELETE CASCADE` FK back to it, so one delete
  removes everything the user owns — live-verified via `psql` (see
  `docs/roadmap.md` Phase 8). Irreversible; the client is expected to
  confirm with the user first (the Flutter app's More screen requires
  typing "DELETE").

## Core ledger (Phase 3)

```
POST   /accounts              {name, currency}                       -> 201 account
GET    /accounts                                                       -> 200 [account] (each with balanceMinor)
GET    /accounts/{id}                                                  -> 200 account
PATCH  /accounts/{id}         {name?, archived?}                      -> 200 account
DELETE /accounts/{id}                                                  -> 204, or 409 if any transaction still references it

POST   /categories            {name, kind: "income"|"expense"}        -> 201 category
GET    /categories                                                     -> 200 [category]
PATCH  /categories/{id}       {name?, archived?}                      -> 200 category
DELETE /categories/{id}                                                -> 204, or 409 if any transaction still references it

POST   /transactions          {accountId, categoryId, direction,
                                amountMinor, currency, occurredAt (YYYY-MM-DD), note?} -> 201 transaction
GET    /transactions          ?accountId=&categoryId=&from=&to=&limit=&offset= -> 200 {items, total, limit, offset}
GET    /transactions/{id}                                              -> 200 transaction
PATCH  /transactions/{id}     any of the create fields, partial        -> 200 transaction
DELETE /transactions/{id}                                              -> 204 (soft delete; excluded from balance/lists after)
```

All ten routes require `Authorization: Bearer <accessToken>`. Every row is
scoped by the caller's user ID at the query level — asking for another user's
account/category/transaction ID returns 404, never 403 (`docs/business-rules.md`).
Amounts are `amountMinor`, a fixed-point integer in the currency's minor units
(never a float). A transaction's `direction` must match its category's `kind`;
an account or category must not be archived to post a new transaction against
it. Archiving (`{"archived": true}` via PATCH) is how you retire an
account/category that still has history — a hard `DELETE` only succeeds when
nothing references it.

## AI receipt scanning (Phase 4)

```
POST /receipts/scan   multipart/form-data, one "image" file field (JPEG/PNG/WebP, max 8MB)
                       -> 200 {merchant, occurredAt, currency, totalMinor, categoryGuess, lineItems[], confidence}
                       -> 429 daily scan limit reached (RECEIPT_SCAN_DAILY_LIMIT_PER_USER, default 10/user/day)
                       -> 422 the model couldn't read it — fall back to the manual entry form
                       -> 503 AI_API_KEY is not configured on this server
```

This endpoint **never saves a transaction**. It calls an OpenAI-compatible
chat-completions gateway (`internal/receipts.OpenAIScanner`; `AI_ENDPOINT`,
`AI_API_KEY`, `AI_MODEL` — currently **Google Gemini**, model
`gemini-flash-latest`, via Gemini's OpenAI-compatibility layer), asking for a
plain JSON reply (not the API's tool-calling feature — see the ADR for why:
mirrors `TaskManagement.Infrastructure.Ai.ChatAiAssistant` elsewhere in this
workspace, which found prompted JSON more reliable across OpenAI-compatible
providers), and returns a draft for the app to pre-fill into the existing
`POST /transactions` flow (Phase 3) — the user still reviews and presses
Save, exactly like manual entry. The original photo is never persisted on the
server: it is held in memory for one API call and discarded. Only a
metadata-only audit row (user, ok/failed, an error code — never the
extracted content) is kept in `receipt_scans`, solely to enforce the daily
limit and see failure patterns; see `docs/adr/0002-ai-receipt-scanning.md`.

**✅ Live-verified working, 2026-09-13**: registered a real user, posted a
real receipt photo to `/receipts/scan` against the real Gemini API, and got
back the correct merchant, date, currency, total, category guess, and every
line item (confidence 0.98). This is the second provider tried — the
first, Biznet Gio's `openai/gpt-oss-20b`, was live-tested and confirmed to
never process images at all (see the ADR for that transcript). Getting
Gemini working also surfaced two real bugs, both fixed: `max_tokens` was too
low for a reasoning model's hidden "thinking" tokens (1500 → 6000,
`internal/receipts.maxCompletionTokens`), and the server's `WriteTimeout`
(15s) was too short for a real AI round trip and silently killed the
connection (60s now, `internal/server.New`). Full details, including the
exact failed/working request-response pairs, are in
`docs/adr/0002-ai-receipt-scanning.md`.

## Reports (Phase 5)

```
GET /reports/summary?from=&to=          -> {from, to, incomeMinor, expenseMinor, netMinor, transactionCount}
GET /reports/by-category?from=&to=      -> {categories: [{categoryId, categoryName, kind, totalMinor, transactionCount}]}
GET /reports/monthly-trend?months=N     -> [{month: "YYYY-MM", incomeMinor, expenseMinor, netMinor}, ...]
```

All three require `Authorization: Bearer <accessToken>` and are scoped to the
caller. `from`/`to` are `YYYY-MM-DD`; when both are omitted, `summary` and
`by-category` default to "this month so far" — `to` before `from` is a 400.
`monthly-trend` defaults to the last 6 months (max 24) ending with the
current month and always returns exactly that many points, oldest first,
**including months with no activity** (zeroed, not omitted) so a chart's
x-axis stays continuous. This package reads `internal/ledger`'s tables
directly (`transactions`, `categories`) — it owns no tables and no migration
of its own.

## Ops & hardening (Phase 7)

```
GET /metrics   -> Prometheus text format: cashflow_http_requests_total{method,route,status},
                  cashflow_http_request_duration_seconds{method,route}. Not behind auth
                  (standard for infra-scraped endpoints) — exposes only request counts/
                  timings, never user data. "route" collapses UUID path segments to "{id}"
                  so cardinality stays bounded no matter how many rows exist.
```

- **Rate limiting** (`internal/server/ratelimit.go`): a strict per-IP limiter
  (2/min sustained, burst 5) on `POST /auth/register` and `POST /auth/login`
  specifically — the two endpoints credential-stuffing/signup-spam bots
  target — plus a much looser one (10/s, burst 20) across every route as a
  general backstop. In-memory, per-instance; if this ever runs behind more
  than one instance, that state needs to move somewhere shared. Exceeding a
  limit returns 429.
- **Request body size cap**: every request body is capped at 9 MiB
  (`internal/server.maxRequestBodyBytes`) server-wide — comfortably above
  `/receipts/scan`'s own tighter 8 MB image-specific limit, so it never
  interferes with a real upload; it exists purely to bound the JSON
  endpoints, which had no cap at all before this.
- **Backup/restore**: `docs/ops/backup-restore.md` plus `scripts/backup.sh`
  (`pg_dump`, custom format, prunes backups older than 30 days) and
  `scripts/restore.sh` (`pg_restore --clean`, requires typing the target
  database name to confirm). Both have actually been run against the local
  Postgres — see that doc for what was verified, not just written.
- **CI** (`.github/workflows/ci.yml`): on every push/PR, a `backend` job runs
  `go build`/`go vet`/`gofmt -l`/`go test`, and a `mobile` job runs
  `flutter create` (checking it doesn't modify committed source, matching
  `mobile/README.md`'s own manual instruction), `flutter analyze`, and
  `flutter test` on GitHub's runners — the first real compiler check any of
  the Flutter code gets, since none exists in the environment that wrote it.

## Run locally

```bash
cp ../env.example ../.env   # from the repo root; fill in local values
docker compose up --build   # from the repo root: Postgres + this API
```

Or without Docker (needs a Postgres reachable at `DATABASE_URL`):

```bash
go run ./cmd/api
```

## Verify

```bash
go build ./...
go vet ./...
go test ./...
```

All three run clean against Go 1.22 (**101 tests** as of Phase 7) and the
whole stack has been verified end-to-end via `docker compose up --build`
against a real Postgres container: migrations (including `0003_ledger` and
`0004_receipt_scans`), register (seeds 6 default categories) → login →
refresh rotation → logout, then create account → post income + expense
transactions → running balance correct → category-kind mismatch rejected
(400) → archived-account posting rejected (409) → deleting an
account/category still in use rejected (409) → soft-deleting a transaction
removes it from the balance and list → a second user can neither read the
first user's account by ID (404) nor see their categories → `/receipts/scan`
without a token (401), without `AI_API_KEY` configured (503), with an
unsupported content type or missing file (400), **and, with a real
`AI_API_KEY`, a real receipt photo through the real Gemini API returning a
fully correct draft** — a direct `psql` check separately confirmed the audit
table stores only status/error_code, never anything extracted from a receipt
→ posting real income/expense transactions across two months and confirming
`/reports/summary` (both the "this month" default and an explicit range),
`/reports/by-category`, and `/reports/monthly-trend` (including a correctly
zero-filled month with no activity) all match by hand, plus the
invalid-range 400 → hammering `/auth/register` past its burst limit and
confirming both the 429s and the underlying successes show up correctly in
`/metrics` → registering, creating owned rows, calling `DELETE /auth/me`,
and confirming via direct `psql` counts that the user, account, all 6
categories, and the refresh token all went from 1 to 0, with the now-stale
access token getting a clean 404 from `/auth/me` afterward. See "AI receipt
scanning" above for the full story of what it took to get that live AI call
actually working.
`go build ./...` needs network access once (`go mod download`) to fetch
`github.com/jackc/pgx/v5`, `github.com/golang-jwt/jwt/v5`,
`golang.org/x/crypto` (bcrypt), `golang.org/x/time` (rate limiting), and
`github.com/prometheus/client_golang` (`/metrics`).

## Adding a migration

Add `NNNN_description.up.sql` (and, for documentation / manual rollback,
`NNNN_description.down.sql`) to `migrations/`, using the next sequential
4-digit prefix so it sorts and applies after the existing ones. The runner
(`internal/platform/db/migrate.go`) applies any `.up.sql` not yet recorded in
`schema_migrations`, each in its own transaction, on every server startup.
