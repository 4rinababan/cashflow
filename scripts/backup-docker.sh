#!/usr/bin/env bash
# Backs up a Cashflow Postgres database that runs as a Docker container with
# no port published to the host (the production deployment's setup — see
# docs/ops/production-deployment.md) by running pg_dump *inside* the
# container itself, then copying the dump out. Same retention behaviour as
# scripts/backup.sh (prunes dumps older than RETENTION_DAYS in BACKUP_DIR),
# reused here rather than duplicated.
#
# Usage: scripts/backup-docker.sh <container_name> <db_user> <db_name> <backup_dir>
# Requires: docker CLI on PATH, and the target container running Postgres.
set -euo pipefail

RETENTION_DAYS="${RETENTION_DAYS:-30}"

if [ $# -ne 4 ]; then
  echo "usage: $0 <container_name> <db_user> <db_name> <backup_dir>" >&2
  exit 1
fi

CONTAINER="$1"
DB_USER="$2"
DB_NAME="$3"
BACKUP_DIR="$4"

mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
REMOTE_TMP="/tmp/cashflow-backup-$TIMESTAMP.dump"
OUT_FILE="$BACKUP_DIR/cashflow-$TIMESTAMP.dump"

echo "Backing up $DB_NAME (container $CONTAINER) to $OUT_FILE ..."
docker exec "$CONTAINER" pg_dump --format=custom -U "$DB_USER" -d "$DB_NAME" -f "$REMOTE_TMP"
docker cp "$CONTAINER:$REMOTE_TMP" "$OUT_FILE"
docker exec "$CONTAINER" rm -f "$REMOTE_TMP"
echo "Backup complete: $(du -h "$OUT_FILE" | cut -f1)"

echo "Pruning backups older than $RETENTION_DAYS days in $BACKUP_DIR ..."
find "$BACKUP_DIR" -name 'cashflow-*.dump' -mtime "+$RETENTION_DAYS" -print -delete
