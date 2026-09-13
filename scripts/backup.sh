#!/usr/bin/env bash
# Backs up a Cashflow Postgres database with pg_dump (custom format), and
# prunes backups in the target directory older than RETENTION_DAYS.
#
# Usage: scripts/backup.sh <DATABASE_URL> <BACKUP_DIR>
# Requires: pg_dump on PATH, matching the target server's major version
# (a client older than the server can fail; see `pg_dump --version`).
set -euo pipefail

RETENTION_DAYS="${RETENTION_DAYS:-30}"

if [ $# -ne 2 ]; then
  echo "usage: $0 <DATABASE_URL> <BACKUP_DIR>" >&2
  exit 1
fi

DATABASE_URL="$1"
BACKUP_DIR="$2"

mkdir -p "$BACKUP_DIR"

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_FILE="$BACKUP_DIR/cashflow-$TIMESTAMP.dump"

echo "Backing up to $OUT_FILE ..."
pg_dump --format=custom --file="$OUT_FILE" "$DATABASE_URL"
echo "Backup complete: $(du -h "$OUT_FILE" | cut -f1)"

echo "Pruning backups older than $RETENTION_DAYS days in $BACKUP_DIR ..."
find "$BACKUP_DIR" -name 'cashflow-*.dump' -mtime "+$RETENTION_DAYS" -print -delete
