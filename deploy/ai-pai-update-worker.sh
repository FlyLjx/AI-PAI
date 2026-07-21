#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

ROOT=${AI_PAI_ROOT:-/opt/ai-pai}
COMPOSE_FILE=${AI_PAI_COMPOSE_FILE:-$ROOT/docker-compose.yml}
ENV_FILE=${AI_PAI_ENV_FILE:-$ROOT/.env}
UPDATE_DIR=${AI_PAI_UPDATE_DIR:-$ROOT/update}
REPOSITORY=${AI_PAI_GITHUB_REPOSITORY:-FlyLjx/AI-PAI}
WORKFLOW=${AI_PAI_GITHUB_WORKFLOW:-build.yml}
REGISTRY=${AI_PAI_REGISTRY:-ghcr.io/flyljx}
REQUEST_FILE=$UPDATE_DIR/request.json
STATUS_FILE=$UPDATE_DIR/status.json
FORCE_FILE=$UPDATE_DIR/force.json
LOCK_FILE=$UPDATE_DIR/update.lock
API_URL="https://api.github.com/repos/$REPOSITORY/actions/workflows/$WORKFLOW/runs?branch=main&status=success&per_page=1"

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
  logger -t ai-pai-update-worker -- "$*" 2>/dev/null || true
}

for command in curl python3 docker flock; do
  command -v "$command" >/dev/null 2>&1 || { log "missing command: $command"; exit 1; }
done
[[ -f "$COMPOSE_FILE" ]] || { log "compose file does not exist"; exit 1; }
[[ -f "$ENV_FILE" ]] || { log "environment file does not exist"; exit 1; }
mkdir -p "$UPDATE_DIR"
chmod 700 "$UPDATE_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log "another update is already running"; exit 0; }
[[ -f "$REQUEST_FILE" ]] || exit 0

request_values=$(python3 - "$REQUEST_FILE" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    request = json.load(handle)
print(
    int(request["runId"]),
    int(request["runNumber"]),
    request["version"],
    request["commit"],
    request.get("currentVersion", "unknown"),
    "true" if request.get("force") else "false",
)
PY
)
read -r run_id run_number version commit current_version force_update <<<"$request_values"
force_update=${force_update:-false}
rm -f "$REQUEST_FILE"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

started_at=$(date --iso-8601=seconds)
pending_task_count=0
write_status() {
  STATUS_VALUE=$1 \
  MESSAGE_VALUE=$2 \
  TARGET_VERSION=$version \
  TARGET_RUN_ID=$run_id \
  TARGET_COMMIT=$commit \
  FORCE_UPDATE=$force_update \
  PENDING_TASK_COUNT=${pending_task_count:-0} \
  STARTED_AT=$started_at \
  FINISHED_AT=${3:-} \
  python3 - "$STATUS_FILE" <<'PY'
import json, os, sys, tempfile
path = sys.argv[1]
payload = {
    "status": os.environ["STATUS_VALUE"],
    "targetVersion": os.environ["TARGET_VERSION"],
    "targetRunId": int(os.environ["TARGET_RUN_ID"]),
    "targetCommit": os.environ["TARGET_COMMIT"],
    "message": os.environ["MESSAGE_VALUE"],
    "startedAt": os.environ["STARTED_AT"],
}
if os.environ.get("FORCE_UPDATE") == "true":
    payload["force"] = True
try:
    pending = int(os.environ.get("PENDING_TASK_COUNT") or "0")
except ValueError:
    pending = 0
if pending > 0:
    payload["pendingTaskCount"] = pending
if os.environ.get("FINISHED_AT"):
    payload["finishedAt"] = os.environ["FINISHED_AT"]
directory = os.path.dirname(path)
fd, temporary = tempfile.mkstemp(prefix=".status-", suffix=".tmp", dir=directory)
try:
    with os.fdopen(fd, "w", encoding="utf-8") as handle:
        json.dump(payload, handle, ensure_ascii=False, indent=2)
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, path)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
PY
}

rollback_needed=false
previous_web=""
previous_admin=""
previous_api=""

set_env_value() {
  local key=$1
  local value=$2
  local temporary
  temporary=$(mktemp "$ROOT/.env.manual-update.XXXXXX")
  awk -v key="$key" -v value="$value" '
    BEGIN { found = 0 }
    index($0, key "=") == 1 { print key "=" value; found = 1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$ENV_FILE" > "$temporary"
  chmod --reference="$ENV_FILE" "$temporary" 2>/dev/null || chmod 600 "$temporary"
  chown --reference="$ENV_FILE" "$temporary" 2>/dev/null || true
  mv -f "$temporary" "$ENV_FILE"
}

handle_failure() {
  local status=$?
  trap - ERR
  set +e
  if [[ "$rollback_needed" == true ]]; then
    write_status rolling_back "Update failed; restoring the previous application images."
    log "$version failed; restoring previous images"
    set_env_value WEB_IMAGE "$previous_web"
    set_env_value ADMIN_IMAGE "$previous_admin"
    set_env_value API_IMAGE "$previous_api"
    export WEB_IMAGE="$previous_web"
    export ADMIN_IMAGE="$previous_admin"
    export API_IMAGE="$previous_api"
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-deps --no-build \
      --force-recreate --wait --wait-timeout 240 api admin ai-pai
  fi
  write_status failed "Update failed. Previous application images were restored; the database container was not changed." "$(date --iso-8601=seconds)"
  exit "$status"
}
trap handle_failure ERR

abort_update() {
  log "$1"
  write_status failed "$1" "$(date --iso-8601=seconds)"
  exit 1
}

active_task_count() {
  local db_driver=${DB_DRIVER:-postgres}
  local db_container=${AI_PAI_DB_CONTAINER:-ai-pai-postgres}
  if [[ "$db_driver" != "postgres" && "$db_driver" != "pgx" ]]; then
    printf '0\n'
    return 0
  fi
  docker exec -e PGPASSWORD="${DB_PASSWORD:-ai_pai_change_me}" "$db_container" \
    psql -U "${DB_USER:-ai_pai}" -d "${DB_NAME:-ai_pai}" -tA \
    -c "SELECT COUNT(*) FROM generation_tasks WHERE status IN ('queued','pending','processing');" \
    | tr -d '[:space:]'
}

wait_for_idle_tasks() {
  if [[ "$force_update" == "true" ]]; then
    pending_task_count=0
    return 0
  fi

  local poll_seconds=${AI_PAI_UPDATE_IDLE_POLL_SECONDS:-15}
  [[ "$poll_seconds" =~ ^[0-9]+$ && "$poll_seconds" -ge 3 ]] || poll_seconds=15

  while true; do
    if [[ -f "$FORCE_FILE" ]]; then
      rm -f "$FORCE_FILE"
      force_update=true
      pending_task_count=0
      write_status queued "Force update signal received; skipping the task idle wait for $version."
      log "force update signal received for $version"
      return 0
    fi
    pending_task_count=$(active_task_count 2>/dev/null || printf 'unknown')
    if [[ "$pending_task_count" =~ ^[0-9]+$ && "$pending_task_count" -eq 0 ]]; then
      return 0
    fi
    if [[ ! "$pending_task_count" =~ ^[0-9]+$ ]]; then
      pending_task_count=0
      write_status waiting_idle "Waiting for task status check before applying $version."
      log "waiting for task status check before applying $version"
    else
      write_status waiting_idle "Waiting for $pending_task_count active generation task(s) to finish before applying $version."
      log "waiting for $pending_task_count active generation task(s) before applying $version"
    fi
    sleep "$poll_seconds"
  done
}

[[ "$run_id" =~ ^[0-9]+$ ]] || abort_update "Invalid Actions run id."
[[ "$run_number" =~ ^[0-9]+$ ]] || abort_update "Invalid Actions run number."
[[ "$version" == "build-$run_number" ]] || abort_update "Version does not match the Actions run number."
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || abort_update "Invalid build commit."

wait_for_idle_tasks

write_status checking "Validating the selected GitHub Actions build."
metadata=$(curl -fsSL --retry 5 --retry-delay 2 \
  -H 'Accept: application/vnd.github+json' \
  -H 'User-Agent: ai-pai-update-worker' \
  "$API_URL")
latest_values=$(python3 -c '
import json, sys
payload = json.load(sys.stdin)
runs = payload.get("workflow_runs") or []
if not runs:
    raise SystemExit("no successful workflow run found")
run = runs[0]
if run.get("conclusion") != "success" or run.get("head_branch") != "main":
    raise SystemExit("latest workflow is not a successful main build")
print(run["id"], run["run_number"], run["head_sha"])
' <<<"$metadata")
read -r latest_id latest_number latest_commit <<<"$latest_values"
[[ "$run_id" == "$latest_id" && "$run_number" == "$latest_number" && "$commit" == "$latest_commit" ]] || {
  abort_update "The selected version is no longer the latest successful Actions build."
}

web_image="$REGISTRY/ai-pai-web:$version"
admin_image="$REGISTRY/ai-pai-admin:$version"
api_image="$REGISTRY/ai-pai-api:$version"
write_status pulling "Pulling Web, Admin, and API images for $version."
for image in "$web_image" "$admin_image" "$api_image"; do
  docker pull "$image"
  docker image inspect "$image" >/dev/null
done

wait_for_idle_tasks

PUBLIC_ORIGIN=${APP_PUBLIC_ORIGIN:-}
previous_web=$(docker inspect ai-pai --format '{{.Config.Image}}')
previous_admin=$(docker inspect ai-pai-admin --format '{{.Config.Image}}')
previous_api=$(docker inspect ai-pai-api --format '{{.Config.Image}}')

rollback_needed=true
set_env_value WEB_IMAGE "$web_image"
set_env_value ADMIN_IMAGE "$admin_image"
set_env_value API_IMAGE "$api_image"
export WEB_IMAGE="$web_image"
export ADMIN_IMAGE="$admin_image"
export API_IMAGE="$api_image"

write_status updating "Replacing application containers with $version; PostgreSQL will not be changed."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-deps --no-build \
  --force-recreate --wait --wait-timeout 240 api admin ai-pai

if [[ -n "$PUBLIC_ORIGIN" ]]; then
  public_base=${PUBLIC_ORIGIN%/}
  curl -fsS --retry 12 --retry-delay 5 "$public_base/healthz" >/dev/null
  curl -fsS --retry 12 --retry-delay 5 "$public_base/sys-admins/healthz" >/dev/null
fi

rollback_needed=false
trap - ERR
write_status success "$version was deployed successfully." "$(date --iso-8601=seconds)"

log "$version deployed successfully (previous version: $current_version)"
