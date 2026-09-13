# Postgres backup & restore (Phase 7)

No production database exists yet — this is the plan and the scripts to run
it, ready for whenever one does. Nothing here has been run against a real
production database (there isn't one); the scripts have been exercised only
against the local `docker compose` Postgres, described below.

## What to back up

Everything lives in one Postgres database (`cashflow`, per `docker-compose.yml`
/ `env.example`). There is no other durable state server-side: receipt
photos are never stored (see `docs/adr/0002-ai-receipt-scanning.md`), and
`.env`/secrets are not data to back up, they're provisioned separately per
environment.

## Scripts

- `scripts/backup.sh $DATABASE_URL /path/to/backups` — runs `pg_dump` in
  Postgres's custom format (`-Fc`, needed for `pg_restore`'s selective/
  parallel restore) and writes a timestamped file, then deletes anything in
  that directory older than 30 days (a first, deliberately simple retention
  policy — revisit the number once there's a real usage/cost picture to
  base it on).
- `scripts/restore.sh $DATABASE_URL /path/to/backup.dump` — restores one
  dump into the target database with `pg_restore --clean --if-exists`,
  dropping and recreating existing objects first. **Only ever point this at
  a database you mean to overwrite.**

Both take the connection as a `postgres://` URL (the same value as
`DATABASE_URL`) so they work the same way locally or against any future
managed Postgres.

## Recommended schedule (once a production database exists)

- A daily `pg_dump` (the script above), retained 30 days, stored somewhere
  other than the database's own disk (e.g. object storage) — a backup next
  to the thing it backs up survives none of the failures that actually
  destroy databases.
- A monthly **restore drill**: actually run `restore.sh` against a scratch
  database and confirm the app works against it. An untested backup is a
  hope, not a plan.
- Whichever managed Postgres provider is eventually chosen (Phase 8 hosting
  decision, not made yet — see `docs/roadmap.md`) likely offers automated
  point-in-time recovery; if so, that supersedes the daily cron above, but
  the monthly restore drill still applies regardless of provider.

## Verified so far

`scripts/backup.sh` and `scripts/restore.sh` were run for real against the
local `docker compose` Postgres (2026-09-12): seeded a marker row, ran
`backup.sh` to produce a dump, restored it with `restore.sh` into a separate
scratch database, and confirmed the exact same row came back — then dropped
the scratch database and cleaned up. Not yet run against a production
database, since none exists.
