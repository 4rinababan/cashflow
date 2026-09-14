# Production deployment

First deployed 2026-09-13; reworked 2026-09-14 to a proper git-based,
self-hosted deploy model (no GitHub Actions — see "Why not CI/CD" below).
Read this before touching the VPS again — the Caddy wiring in particular is
not obvious from the code alone.

## Where it runs

- **VPS**: `103.89.7.185`, user `cashflow`, SSH key `~/.ssh/cahsflow.pem` (on
  the machine that manages this deployment). Ubuntu 22.04, Docker + Docker
  Compose installed.
- **This is a shared VPS.** It already runs a separate, unrelated production
  app (`gawean-management` / TaskManagement) under a Docker Compose project
  named `deploy` — containers `deploy-web-1`, `deploy-db-1`,
  `deploy-db-backup-1`, `deploy-caddy-1`. Cashflow's own stack is a
  completely separate Compose project (`cashflow`, directory `~/cashflow/`
  on the VPS) with its own Postgres, network, and volume — nothing is
  shared with that other app except the reverse proxy (see below), and
  resource limits (see "Resource isolation") keep it from being able to
  starve that app even by accident. Do not assume this VPS is dedicated to
  Cashflow.
- **Code layout on the VPS**: `~/cashflow/` is a real `git clone` of
  <https://github.com/4rinababan/cashflow> (private repo) — not an `rsync`
  target anymore (that was the original, first-cut approach; superseded the
  next day once a proper deploy flow was worth setting up). `~/cashflow/.env`
  (mode 600, git-ignored) holds production secrets.
- **Domain**: `https://api.laburaku.com` → this VPS. DNS is managed
  separately from this deployment (DNS Management panel for `laburaku.com`).

## Why not CI/CD

GitHub Actions is not used — the GitHub account's billing is locked
("GitHub Actions workflows can't be executed on this repository"), which
blocks **every** Actions run account-wide, hosted or self-hosted (a
self-hosted runner was registered and tested specifically to check this; it
made no difference — GitHub refuses to even schedule a run while billing is
locked). Rather than wait on that, deployment is plain `git pull` +
`docker compose`, run by hand over SSH — see "Deploying" below. If billing
ever gets sorted, `.github/workflows/*.yml` still exist and are valid, but
nothing currently depends on them.

## Deploying

```bash
ssh -i ~/.ssh/cahsflow.pem cashflow@103.89.7.185
~/cashflow/scripts/deploy-vps.sh
```

That one script does everything: `git fetch` + hard-reset to
`origin/master`, rebuild + restart the stack with the production override,
print status, and re-apply the shared Caddy route (idempotent — see
below). It's meant to be safe to re-run if it fails partway.

**GitHub access**: the VPS has its own SSH keypair
(`~/.ssh/github_cashflow_deploy`) registered as a **read-only deploy key**
on the repo (Settings → Deploy keys) — not a personal token, and it can
only pull this one repo, nothing else on the account. `~/.ssh/config` on
the VPS points `github.com` at that key.

## Resource isolation

`docker-compose.prod.yml` caps `api` at 256MB RAM / 1 CPU and `postgres` at
512MB RAM / 1 CPU (`deploy.resources.limits`, honored by plain
`docker compose up`, not just Swarm). The VPS has 3.8GB RAM total and the
other app's whole stack normally uses well under 500MB, so this leaves
comfortable headroom for both — the actual point of the limits is a hard
ceiling: a leak or a runaway request storm in Cashflow can eat at most
768MB, never the whole box, so it can never take `gawean-management` down
as a side effect. Check current usage any time with `docker stats
--no-stream`; check the limits actually took with
`docker inspect cashflow-api-1 --format '{{.HostConfig.Memory}} {{.HostConfig.NanoCpus}}'`.

## Networking: why there's no public port, and how HTTPS works

`docker-compose.prod.yml` overrides both `postgres` and `api` to publish
**no ports at all** (`ports: !reset []` — a plain `ports: []` does NOT work,
Compose merges list values from the base file by default; `!reset` is
required to actually clear it). Both services are reachable only from
inside the `cashflow_default` Docker network. Always deploy with both
compose files together (`deploy-vps.sh` already does this):

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Deploying with the override forgotten (`docker compose up -d --build`
alone) will publish Postgres and the API directly to the public internet on
their default ports — this happened once during the very first deploy and
was caught and fixed within a couple of minutes, but check the `PORTS`
column of `docker compose ps` after every manual deploy to be sure (it
should show bare `8080/tcp` / `5432/tcp`, never `0.0.0.0:...`).

HTTPS for `api.laburaku.com` is handled by the **existing shared Caddy**
(`deploy-caddy-1`), not a Caddy of Cashflow's own — there's nowhere else for
it to go, since that container already owns host ports 80/443. Two things
make this work, both deliberately *not* touching anything that belongs to
the `gawean-management` project:

1. `docker network connect cashflow_default deploy-caddy-1` — attaches
   the shared Caddy container to Cashflow's Docker network (in addition to
   its own), so it can reach the `api` container by name. Safe, reversible
   (`docker network disconnect cashflow_default deploy-caddy-1`),
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
own site block.

**This is handled automatically now**, not just documented as a manual
fix: `scripts/reapply-caddy-route.sh` is idempotent (checks whether the
route is already present before touching anything) and runs from cron
every 10 minutes:

```
*/10 * * * * /home/cashflow/cashflow/scripts/reapply-caddy-route.sh >> /home/cashflow/cashflow-monitor/caddy-route.log 2>&1
```

So a Caddy restart causes at most a ~10-minute gap, self-heals with no one
having to notice or intervene, and `monitor.py` (below) would catch and
email about the gap regardless. To force it immediately instead of waiting
for cron: `~/cashflow/scripts/reapply-caddy-route.sh`.

A more durable long-term fix would be for `gawean-management`'s own
Caddyfile to `import` an external, not-CI-managed directory of site
configs — but that's a change to a *different* project's deployment
pipeline and is out of scope here; flag it to whoever owns that project if
this keeps being a recurring annoyance.

## Secrets

`~/cashflow/.env` on the VPS (mode 600, git-ignored, never committed) holds
the real production values: `JWT_SIGNING_KEY` and `POSTGRES_PASSWORD` were
generated directly on the VPS with `openssl rand` (never displayed
anywhere, including in this chat/repo); `AI_API_KEY` (Gemini) and
`SMTP_PASSWORD` were provided by the user for this deployment specifically.
`APP_BASE_URL=https://api.laburaku.com` (used to build workspace-invite
links). This file is **not** part of the git clone and is **not**
recreated by `deploy-vps.sh` — it was set up once by hand and survives
every `git reset --hard` deploy untouched (it's git-ignored, so a hard
reset never touches it).

## Verified working

`https://api.laburaku.com/healthz` and `/readyz` both 200 with a real
Let's Encrypt certificate (issued automatically by Caddy once DNS
resolved); a full register → personal workspace → `DELETE /auth/me`
round trip was run against the live production instance to confirm
everything works end to end, then the test account was deleted again. The
`gawean-management` stack was confirmed still `Up ... (healthy)`,
untouched, throughout — including through the 2026-09-14 rework (stack
moved from the `cashflow-deploy` project/directory to `cashflow`, verified
`SELECT count(*) FROM users` was 0 before the switch so nothing was lost,
and the public URL was re-tested immediately after).

## Backups

`scripts/backup-docker.sh` (a container-aware variant of
`scripts/backup.sh` — Postgres here has no port published to the host, so
`pg_dump` has to run *inside* the container via `docker exec`, not connect
over the network) is scheduled via cron:

```
0 3 * * * /home/cashflow/cashflow/scripts/backup-docker.sh cashflow-postgres-1 cashflow cashflow /home/cashflow/cashflow-backups >> /home/cashflow/cashflow-backups/backup.log 2>&1
```

Daily at 03:00 Asia/Jakarta (the VPS's local time). Dumps land in
`~/cashflow-backups/cashflow-<UTC-timestamp>.dump` (pg_dump custom format —
restore with `scripts/restore.sh`, or manually via
`pg_restore --clean --if-exists --no-owner` against a running Postgres),
pruned after 30 days (`RETENTION_DAYS` env var on the script, same as the
non-Docker variant). Verified working (ran it manually, confirmed the
dump's `PGDMP` magic header via `od -c`; re-verified again after the
2026-09-14 container rename).

Restoring a dump into the live container follows the same "run pg_restore
inside the container" pattern: `docker cp` the dump file in, then
`docker exec cashflow-postgres-1 pg_restore --clean --if-exists
--no-owner -U cashflow -d cashflow /path/inside/container.dump`. Not yet
scripted as its own `restore-docker.sh` — do that before the first time you
actually need it under pressure, not during an incident.

## Monitoring

`scripts/monitor.py` checks `GET /healthz` and `/readyz` on the public URL
every 5 minutes via cron:

```
*/5 * * * * /usr/bin/python3 /home/cashflow/cashflow/scripts/monitor.py >> /home/cashflow/cashflow-monitor/monitor.log 2>&1
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
`python3 ~/cashflow/scripts/monitor.py --test-email`. There is no dashboard —
this is deliberately just "email me if it breaks," not a metrics UI (the
app's own `GET /metrics` Prometheus endpoint exists for that if a proper
Grafana/Prometheus setup is ever wanted later).

## Everyday operations cheat sheet

| Want to... | Run |
| --- | --- |
| Deploy the latest `master` | `~/cashflow/scripts/deploy-vps.sh` |
| Check both apps are healthy | `docker ps --format 'table {{.Names}}\t{{.Status}}'` |
| Check resource usage right now | `docker stats --no-stream` |
| Force the Caddy route to reapply | `~/cashflow/scripts/reapply-caddy-route.sh` |
| Back up the database right now | `~/cashflow/scripts/backup-docker.sh cashflow-postgres-1 cashflow cashflow ~/cashflow-backups` |
| Test the monitoring email path | `python3 ~/cashflow/scripts/monitor.py --test-email` |
| Read backend logs | `docker compose -f ~/cashflow/docker-compose.yml -f ~/cashflow/docker-compose.prod.yml logs -f api` |

## Not yet done

- No `restore-docker.sh` companion script yet (see Backups above).
- Monitoring is a single email alert, not a dashboard — fine for one person
  watching one small deployment, revisit if that stops being true.
- If GitHub billing ever gets fixed, decide whether to switch back to
  Actions-driven deploys or keep this git-pull model (it works, is simpler
  to reason about, and has no billing dependency at all — there's a real
  argument for keeping it either way).
