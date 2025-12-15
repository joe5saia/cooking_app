#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

COMPOSE_FILE=${COMPOSE_FILE:-"$SCRIPT_DIR/compose.yaml"}
BACKUP_DIR=${BACKUP_DIR:-"$SCRIPT_DIR/backups"}
KEEP_DAILY=${KEEP_DAILY:-14}

mkdir -p "$BACKUP_DIR"

timestamp=$(date -u +"%Y%m%dT%H%M%SZ")
out="$BACKUP_DIR/cooking_app_pg_dump_${timestamp}.sql.gz"

echo "Writing backup to $out" >&2

docker compose -f "$COMPOSE_FILE" exec -T db sh -lc 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip -c >"$out"

echo "Applying retention: keep last $KEEP_DAILY" >&2
to_delete=$(ls -1t "$BACKUP_DIR"/cooking_app_pg_dump_*.sql.gz 2>/dev/null | tail -n "+$((KEEP_DAILY + 1))" || true)
if [ -n "$to_delete" ]; then
  echo "$to_delete" | xargs rm -f
fi
