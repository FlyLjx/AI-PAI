#!/usr/bin/env bash
set -Eeuo pipefail

# Reclaim generation_tasks heap/index space without VACUUM FULL's long table lock.
# Run this after the API cleanup status reports state=completed.

umask 077

PGHOST=${PGHOST:-127.0.0.1}
PGPORT=${PGPORT:-5432}
PGUSER=${PGUSER:-ai_pai}
PGDATABASE=${PGDATABASE:-ai_pai}
PGREPACK_BIN=${PGREPACK_BIN:-pg_repack}
PGREPACK_WAIT_TIMEOUT=${PGREPACK_WAIT_TIMEOUT:-60}
TABLE=${PGREPACK_TABLE:-public.generation_tasks}

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

for command in psql "$PGREPACK_BIN"; do
  command -v "$command" >/dev/null 2>&1 || {
    log "missing command: $command"
    exit 1
  }
done

psql_args=(--no-psqlrc --tuples-only --quiet --host "$PGHOST" --port "$PGPORT" --username "$PGUSER" --dbname "$PGDATABASE")

relation_size() {
  psql "${psql_args[@]}" -c "SELECT pg_size_pretty(pg_total_relation_size('$TABLE'));" | tr -d '[:space:]'
}

base64_rows() {
  psql "${psql_args[@]}" -c "
    SELECT COUNT(*)
    FROM $TABLE
    WHERE result_json IS NOT NULL
      AND (result_json::text ILIKE '%b64_json%' OR result_json::text ILIKE '%base64%');
  " | tr -d '[:space:]'
}

active_tasks() {
  psql "${psql_args[@]}" -c "
    SELECT COUNT(*)
    FROM $TABLE
    WHERE status IN ('queued', 'pending', 'processing');
  " | tr -d '[:space:]'
}

before_size=$(relation_size)
before_base64=$(base64_rows)
running=$(active_tasks)
log "table=$TABLE size_before=$before_size base64_rows=$before_base64 active_tasks=$running"

if [[ "$running" != "0" ]]; then
  log "active generation tasks detected; wait for cleanup and retry"
  exit 2
fi

log "running pg_repack without a full-table lock"
"$PGREPACK_BIN" \
  --no-order \
  --wait-timeout="$PGREPACK_WAIT_TIMEOUT" \
  --table="$TABLE" \
  --host="$PGHOST" \
  --port="$PGPORT" \
  --username="$PGUSER" \
  --dbname="$PGDATABASE"

psql "${psql_args[@]}" -c "VACUUM (ANALYZE) $TABLE;" >/dev/null

after_size=$(relation_size)
after_base64=$(base64_rows)
log "table=$TABLE size_after=$after_size base64_rows=$after_base64"

if [[ "$after_base64" != "0" ]]; then
  log "warning: Base64 rows remain; check /api/admin/image-cleanup/status and deploy the updated API"
  exit 3
fi
