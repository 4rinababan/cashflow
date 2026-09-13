#!/usr/bin/env bash
# Restores a Cashflow Postgres database from a pg_dump custom-format file,
# dropping and recreating existing objects first (--clean --if-exists).
#
# Usage: scripts/restore.sh <DATABASE_URL> <DUMP_FILE>
#
# DANGER: this overwrites whatever is currently in the target database.
# Only ever point it at a database you mean to replace — a scratch/restore-
# drill database, or a real one you are deliberately recovering.
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <DATABASE_URL> <DUMP_FILE>" >&2
  exit 1
fi

DATABASE_URL="$1"
DUMP_FILE="$2"

if [ ! -f "$DUMP_FILE" ]; then
  echo "dump file not found: $DUMP_FILE" >&2
  exit 1
fi

echo "Restoring $DUMP_FILE into $DATABASE_URL ..."
echo "This will drop and recreate existing objects in the target database."
read -r -p "Type the target database name to confirm: " CONFIRM
TARGET_DB="$(echo "$DATABASE_URL" | sed -E 's#.*/([^/?]+).*#\1#')"
if [ "$CONFIRM" != "$TARGET_DB" ]; then
  echo "Confirmation did not match database name ($TARGET_DB) — aborting." >&2
  exit 1
fi

pg_restore --clean --if-exists --no-owner --dbname="$DATABASE_URL" "$DUMP_FILE"
echo "Restore complete."
