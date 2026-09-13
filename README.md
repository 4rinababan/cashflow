# Cashflow

A personal income/expense tracker with AI-assisted receipt scanning, built to be
released free on the Google Play Store for many independent users.

**Flutter** mobile app + **Go** backend API + **PostgreSQL**. Each user's data is
private to them; there is no cross-user sharing. Receipt photos are parsed by a
vision-capable AI model into a draft transaction the user reviews and confirms —
the AI never posts a transaction on its own.

> **Status: Phases 1–7 done, Phase 8 mostly done.** Backend Phases 1–5, a
> full Flutter UI including the Phase 6 polish pass, Phase 7 ops hardening
> (rate limiting, `/metrics`, backup/restore — actually tested, not just
> written — and real CI), and most of Phase 8 (self-service account
> deletion, a generated app icon, a Play listing draft). See
> [`docs/roadmap.md`](docs/roadmap.md) for the full plan and
> [`docs/adr/`](docs/adr/) for the decisions behind it. **Not yet
> ready for a public release** — what's left is deliberately all human-gated:
> a legal review of the privacy policy, the actual Play Console
> account/signing key/listing (the AI does not hold or create these — see
> `docs/ops/android-signing.md`), and — important, not a formality — **the
> Flutter app has never been built or run** in the environment that wrote it
> (no Flutter/Dart SDK available); see `mobile/README.md` for the
> verification steps to run before trusting it compiles, or push to GitHub
> and let `.github/workflows/ci.yml` do it. Receipt scanning is, as of
> 2026-09-13, **live-verified working on the backend** — a real receipt
> photo through the real Gemini API (`gemini-flash-latest`) came back with a
> fully correct draft, amounts included (see
> `docs/adr/0002-ai-receipt-scanning.md` for that story, including why its
> first provider, Biznet Gio's `openai/gpt-oss-20b`, had to be abandoned —
> confirmed unable to process images at all).

## Repository layout

```
cashflow/
├── backend/     Go API (cmd/, internal/, migrations/)
├── mobile/      Flutter app (lib/, test/)
├── docs/        roadmap, ADRs, business rules, ops runbooks, store listing
├── scripts/     backup.sh / restore.sh (docs/ops/backup-restore.md)
├── .github/     CI (build+test on every push — backend and mobile)
└── docker-compose.yml   Postgres + backend for local dev
```

## Quick start (backend)

```bash
cp env.example .env           # fill in local values; never commit .env
docker compose up --build     # Postgres + the Go API
curl http://localhost:8080/healthz

# register + log in (see backend/README.md for the full endpoint list)
curl -X POST http://localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"correct-horse-battery","displayName":"You"}'

# then, with the accessToken from that response: log a transaction
curl -X POST http://localhost:8080/accounts -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Cash","currency":"IDR"}'
```

Without Docker:

```bash
cd backend
go run ./cmd/api
```

## Quick start (mobile)

The Flutter app has **not been verified to build in this environment** — no
Flutter/Dart SDK was available here. See `mobile/README.md` first — it needs
one manual step (camera/photo permissions) before `flutter run` will let you
scan a receipt. Then:

```bash
cd mobile
flutter create --platforms android,ios --project-name cashflow .   # first time only
flutter pub get
flutter analyze
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

`API_BASE_URL` defaults to the Android emulator's alias for your host machine
(see `mobile/lib/core/env/env.dart`) — a physical device needs your machine's
real LAN IP instead.

## Documentation

| Doc | Contents |
| --- | --- |
| [`docs/roadmap.md`](docs/roadmap.md) | Phased build plan, current status |
| [`docs/adr/0001-architecture.md`](docs/adr/0001-architecture.md) | Stack, layering, multi-user data model |
| [`docs/adr/0002-ai-receipt-scanning.md`](docs/adr/0002-ai-receipt-scanning.md) | How receipt → draft transaction works, cost/privacy tradeoffs |
| [`docs/business-rules.md`](docs/business-rules.md) | Domain rules (categories, currencies, review-before-post) |
| [`docs/privacy-policy.md`](docs/privacy-policy.md) | Draft privacy policy (Bahasa Indonesia) — needs legal review and the `[ISI ...]` placeholders filled in before publishing |
| [`docs/store-listing.md`](docs/store-listing.md) | Play Store listing draft (title, descriptions, app icon, data safety summary) |
| [`docs/ops/backup-restore.md`](docs/ops/backup-restore.md) | Postgres backup/restore plan and scripts — actually tested against local Postgres |
| [`docs/ops/android-signing.md`](docs/ops/android-signing.md) | Android release signing steps (human-run) and the staged rollout plan |

## Safety / privacy notes

- Every ledger row is owned by exactly one user (`user_id`); the API never
  returns or accepts another user's data.
- The AI only **drafts** a transaction from a receipt photo; the user must
  confirm before it is saved. Nothing is posted automatically.
- Receipt images and extracted text may contain personal data — see
  [`docs/adr/0002-ai-receipt-scanning.md`](docs/adr/0002-ai-receipt-scanning.md)
  for retention/redaction notes. A privacy policy draft exists
  ([`docs/privacy-policy.md`](docs/privacy-policy.md)) but needs a legal
  review before a public Play Store listing.
- A user can permanently delete their own account and everything it owns
  from inside the app (`DELETE /auth/me`; see `backend/README.md`) —
  required for a Play Store financial app, and live-verified to actually
  cascade-delete every owned row, not just the user record.
- `.env` is never committed; secrets (DB password, JWT signing key, AI API key)
  are placeholders in `env.example` only. The Android release signing key is
  never generated or held by the AI — see `docs/ops/android-signing.md`.
