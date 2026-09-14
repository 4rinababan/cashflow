#!/usr/bin/env bash
# Idempotently ensures the shared Caddy container (another app's, on the
# same VPS — see docs/ops/production-deployment.md) has a live route for
# api.laburaku.com -> api:8080. Safe to run repeatedly (e.g. from cron as a
# self-healing check) — does nothing if the route is already present.
#
# Why this exists at all: the route lives only in Caddy's in-memory config,
# added via its admin API rather than by editing that other app's own
# Caddyfile (which is managed by ITS OWN CI/CD and would silently discard
# an edit on its next deploy). The cost of that choice is that a restart of
# the shared Caddy container (a host reboot, or the other app's own deploy
# restarting it) drops the route — this script exists to notice and fix
# that automatically rather than requiring someone to remember.
set -euo pipefail

CADDY_CONTAINER="deploy-caddy-1"
CASHFLOW_NETWORK="cashflow-deploy_default"
ROUTE_HOST="api.laburaku.com"
ROUTE_FILE="$(dirname "$0")/../cashflow-caddy-route.json"

if ! docker inspect "$CADDY_CONTAINER" >/dev/null 2>&1; then
  echo "Caddy container ($CADDY_CONTAINER) not found — nothing to do." >&2
  exit 0
fi

docker network connect "$CASHFLOW_NETWORK" "$CADDY_CONTAINER" 2>/dev/null || true

CURRENT_ROUTES="$(docker exec "$CADDY_CONTAINER" curl -s http://127.0.0.1:2019/config/apps/http/servers/srv0/routes)"

if echo "$CURRENT_ROUTES" | grep -q "\"$ROUTE_HOST\""; then
  echo "Route for $ROUTE_HOST already present — nothing to do."
  exit 0
fi

echo "Route for $ROUTE_HOST missing — re-applying."
docker cp "$ROUTE_FILE" "$CADDY_CONTAINER:/tmp/cashflow-route.json"
docker exec "$CADDY_CONTAINER" curl -s -X POST -H 'Content-Type: application/json' \
  --data @/tmp/cashflow-route.json \
  http://127.0.0.1:2019/config/apps/http/servers/srv0/routes
echo "Route re-applied."
