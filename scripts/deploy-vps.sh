#!/usr/bin/env bash
# Deploys Cashflow's backend on the production VPS: pulls the latest master
# from GitHub, then rebuilds and restarts just the backend stack. No CI/CD
# service involved — this is meant to be run directly on the VPS over SSH
# (docs/ops/production-deployment.md). Idempotent: safe to run again if a
# previous run failed partway.
#
# Usage (from anywhere): ~/cashflow/scripts/deploy-vps.sh
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> Pulling latest master..."
git fetch origin master
git reset --hard origin/master

echo "==> Building and restarting the backend stack..."
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

echo "==> Current status:"
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps

echo "==> Re-applying the shared Caddy route (idempotent, no-op if already present)..."
./scripts/reapply-caddy-route.sh || echo "  (skipped — see reapply-caddy-route.sh if this matters)"

echo "==> Done. Verify: curl https://api.laburaku.com/healthz"
