# Production deployment

First production deployment, 2026-09-13. Read this before touching the VPS
again — the Caddy wiring in particular is not obvious from the code alone.

## Where it runs

- **VPS**: `103.89.7.185`, user `cashflow`, SSH key `~/.ssh/cahsflow.pem` (on
  the machine that manages this deployment). Ubuntu 22.04, Docker + Docker
  Compose installed.
- **This is a shared VPS.** It already runs a separate, unrelated production
  app (`gawean-management` / TaskManagement) under a Docker Compose project
  named `deploy` — containers `deploy-web-1`, `deploy-db-1`,
  `deploy-db-backup-1`, `deploy-caddy-1`. Cashflow's own stack is a
  completely separate Compose project (`cashflow-deploy`, directory
  `~/cashflow-deploy/` on the VPS) with its own Postgres, network, and
  volume — nothing is shared with that other app except the reverse proxy
  (see below). Do not assume this VPS is dedicated to Cashflow.
- **Code layout on the VPS**: `~/cashflow-deploy/backend/` (synced via
  `rsync`, not `git clone` — the local repo has no git remote) and
  `~/cashflow-deploy/docker-compose.yml` + `docker-compose.prod.yml`
  (override that removes published host ports — see below).
- **Domain**: `https://api.laburaku.com` → this VPS. DNS is managed
  separately from this deployment (DNS Management panel for `laburaku.com`).

## Networking: why there's no public port, and how HTTPS works

`docker-compose.prod.yml` overrides both `postgres` and `api` to publish
**no ports at all** (`ports: !reset []` — a plain `ports: []` does NOT work,
Compose merges list values from the base file by default; `!reset` is
required to actually clear it). Both services are reachable only from
inside the `cashflow-deploy_default` Docker network. Always deploy with:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Deploying with the override forgotten (`docker compose up -d --build`
alone) will publish Postgres and the API directly to the public internet on
their default ports — this happened once during the first deploy and was
caught and fixed within a couple of minutes, but check the `PORTS` column of
`docker compose ps` after every deploy to be sure (it should show bare
`8080/tcp` / `5432/tcp`, never `0.0.0.0:...`).

HTTPS for `api.laburaku.com` is handled by the **existing shared Caddy**
(`deploy-caddy-1`), not a Caddy of Cashflow's own — there's nowhere else for
it to go, since that container already owns host ports 80/443. Two things
make this work, both deliberately *not* touching anything that belongs to
the `gawean-management` project:

1. `docker network connect cashflow-deploy_default deploy-caddy-1` — attaches
   the shared Caddy container to Cashflow's Docker network (in addition to
   its own), so it can reach the `api` container by name. Safe, reversible
   (`docker network disconnect cashflow-deploy_default deploy-caddy-1`),
   does not restart the container or touch its files.
2. A route for `api.laburaku.com → reverse_proxy api:8080` was added
   directly to Caddy's **running** config via its admin API
   (`http://127.0.0.1:2019/config/...`, reachable only from inside the
   container — `docker exec deploy-caddy-1 curl ...`), **not** by editing
   `deploy-caddy-1`'s actual `Caddyfile`. That file lives at
   `/opt/taskmanagement/actions-runner/_work/gawean-management/gawean-management/deploy/Caddyfile`
   on the host — inside `gawean-management`'s own CI/CD working directory —
   so it gets overwritten by that project's own deploy pipeline. Editing it
   directly would have been fragile and is exactly what this approach
   avoids.

**The consequence**: this route lives only in Caddy's in-memory config. It
is lost if `deploy-caddy-1` is ever restarted (a host reboot, or
`gawean-management`'s own deploy process restarting it) — Caddy reloads
from the Caddyfile on restart, which still only has `gawean-management`'s
own site block. If `https://api.laburaku.com` ever stops working and
`gawean-management` still works fine, **this is almost certainly why**.

**To reapply it**: `~/cashflow-deploy/reapply-caddy-route.sh` on the VPS
does exactly this (idempotent — safe to run even if already applied). It
needs `~/cashflow-route.json` (also already on the VPS) to exist alongside
it; both were left there for this purpose. Run:

```bash
ssh -i ~/.ssh/cahsflow.pem cashflow@103.89.7.185 '~/cashflow-deploy/reapply-caddy-route.sh'
```

A more durable long-term fix would be for `gawean-management`'s own
Caddyfile to `import` an external, not-CI-managed directory of site
configs — but that's a change to a *different* project's deployment
pipeline and is out of scope here; flag it to whoever owns that project if
this keeps being a recurring annoyance.

## Secrets

`~/cashflow-deploy/.env` on the VPS (mode 600, never committed, never
copied from the local dev `.env`) holds the real production values:
`JWT_SIGNING_KEY` and `POSTGRES_PASSWORD` were generated directly on the VPS
with `openssl rand` (never displayed anywhere, including in this chat/repo);
`AI_API_KEY` (Gemini) and `SMTP_PASSWORD` were provided by the user for this
deployment specifically. `APP_BASE_URL=https://api.laburaku.com` (used to
build workspace-invite links).

## Verified working (2026-09-13)

`https://api.laburaku.com/healthz` and `/readyz` both 200 with a real
Let's Encrypt certificate (issued automatically by Caddy once DNS
resolved); a full register → personal workspace → `DELETE /auth/me`
round trip was run against the live production instance to confirm
everything works end to end, then the test account was deleted again. The
`gawean-management` stack was confirmed still `Up ... (healthy)`,
untouched, throughout.

## Backups

`scripts/backup-docker.sh` (a container-aware variant of
`scripts/backup.sh` — Postgres here has no port published to the host, so
`pg_dump` has to run *inside* the container via `docker exec`, not connect
over the network) is deployed to `~/cashflow-deploy/backup-docker.sh` and
scheduled via cron:

```
0 3 * * * /home/cashflow/cashflow-deploy/backup-docker.sh cashflow-deploy-postgres-1 cashflow cashflow /home/cashflow/cashflow-backups >> /home/cashflow/cashflow-backups/backup.log 2>&1
```

Daily at 03:00 Asia/Jakarta (the VPS's local time). Dumps land in
`~/cashflow-backups/cashflow-<UTC-timestamp>.dump` (pg_dump custom format —
restore with `scripts/restore.sh`, or manually via
`pg_restore --clean --if-exists --no-owner` against a running Postgres),
pruned after 30 days (`RETENTION_DAYS` env var on the script, same as the
non-Docker variant). Verified once manually before scheduling (ran it,
confirmed the dump's `PGDMP` magic header via `od -c`).

Restoring a dump into the live container follows the same "run pg_restore
inside the container" pattern: `docker cp` the dump file in, then
`docker exec cashflow-deploy-postgres-1 pg_restore --clean --if-exists
--no-owner -U cashflow -d cashflow /path/inside/container.dump`. Not yet
scripted as its own `restore-docker.sh` — do that before the first time you
actually need it under pressure, not during an incident.

## Monitoring

`scripts/monitor.py` (deployed to `~/cashflow-deploy/monitor.py`) checks
`GET /healthz` and `/readyz` on the public URL every 5 minutes via cron:

```
*/5 * * * * /usr/bin/python3 /home/cashflow/cashflow-deploy/monitor.py >> /home/cashflow/cashflow-monitor/monitor.log 2>&1
```

It emails **ari.decentindonesia@gmail.com** (using the same SMTP account
Cashflow itself uses for workspace invites — no new credentials) only on a
state *transition* (up→down or down→up), tracked in
`~/cashflow-monitor/state` — never on every single check, so a sustained
outage sends exactly one "DOWN" email and one "RECOVERED" email, not one
every 5 minutes. Both the health check and the SMTP send retry up to 3
times, 5 seconds apart, before giving up — this VPS's DNS resolver showed
transient one-off lookup failures a few times during setup (immediately
succeeding on retry), and the retry exists specifically so that never turns
into a false alert.

Test the email path directly, independent of actual up/down state:
`python3 ~/cashflow-deploy/monitor.py --test-email`. There is no dashboard —
this is deliberately just "email me if it breaks," not a metrics UI (the
app's own `GET /metrics` Prometheus endpoint exists for that if a proper
Grafana/Prometheus setup is ever wanted later).

## Not yet done

- No `restore-docker.sh` companion script yet (see Backups above).
- Mobile app is not configured to point at this URL by default — see
  `mobile/README.md`'s `--dart-define=API_BASE_URL=...` instructions; no
  actual Flutter build has been produced or tested against it (no SDK
  available in the environment maintaining this codebase).
- Monitoring is a single email alert, not a dashboard — fine for one person
  watching one small deployment, revisit if that stops being true.
