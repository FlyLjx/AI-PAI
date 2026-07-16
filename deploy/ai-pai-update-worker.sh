#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

ROOT=${AI_PAI_ROOT:-/opt/ai-pai}
COMPOSE_FILE=${AI_PAI_COMPOSE_FILE:-$ROOT/docker-compose.yml}
ENV_FILE=${AI_PAI_ENV_FILE:-$ROOT/.env}
UPDATE_DIR=${AI_PAI_UPDATE_DIR:-$ROOT/update}
BACKUP_ROOT=${AI_PAI_BACKUP_ROOT:-$ROOT/backups/manual-updates}
BACKUP_RETENTION=${AI_PAI_BACKUP_RETENTION:-3}
REPOSITORY=${AI_PAI_GITHUB_REPOSITORY:-FlyLjx/AI-PAI}
WORKFLOW=${AI_PAI_GITHUB_WORKFLOW:-build.yml}
REGISTRY=${AI_PAI_REGISTRY:-ghcr.io/flyljx}
REQUEST_FILE=$UPDATE_DIR/request.json
STATUS_FILE=$UPDATE_DIR/status.json
LOCK_FILE=$UPDATE_DIR/update.lock
API_URL="https://api.github.com/repos/$REPOSITORY/actions/workflows/$WORKFLOW/runs?branch=main&status=success&per_page=1"

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
  logger -t ai-pai-update-worker -- "$*" 2>/dev/null || true
}

for command in curl python3 docker flock sha256sum; do
  command -v "$command" >/dev/null 2>&1 || { log "missing command: $command"; exit 1; }
done
[[ -f "$COMPOSE_FILE" ]] || { log "compose file does not exist"; exit 1; }
[[ -f "$ENV_FILE" ]] || { log "environment file does not exist"; exit 1; }
[[ "$BACKUP_RETENTION" =~ ^[1-9][0-9]*$ ]] || { log "invalid backup retention"; exit 1; }

mkdir -p "$UPDATE_DIR" "$BACKUP_ROOT"
chmod 700 "$UPDATE_DIR" "$BACKUP_ROOT"
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
)
PY
)
read -r run_id run_number version commit current_version <<<"$request_values"
rm -f "$REQUEST_FILE"

started_at=$(date --iso-8601=seconds)
backup_dir=""
write_status() {
  STATUS_VALUE=$1 \
  MESSAGE_VALUE=$2 \
  TARGET_VERSION=$version \
  TARGET_RUN_ID=$run_id \
  TARGET_COMMIT=$commit \
  STARTED_AT=$started_at \
  FINISHED_AT=${3:-} \
  BACKUP_DIRECTORY=$backup_dir \
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
if os.environ.get("FINISHED_AT"):
    payload["finishedAt"] = os.environ["FINISHED_AT"]
if os.environ.get("BACKUP_DIRECTORY"):
    payload["backupDirectory"] = os.environ["BACKUP_DIRECTORY"]
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
    docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-build \
      --force-recreate --wait --wait-timeout 240 api admin ai-pai
  fi
  write_status failed "Update failed. Previous application images were restored; the database backup was retained." "$(date --iso-8601=seconds)"
  exit "$status"
}
trap handle_failure ERR

abort_update() {
  log "$1"
  write_status failed "$1" "$(date --iso-8601=seconds)"
  exit 1
}

[[ "$run_id" =~ ^[0-9]+$ ]] || abort_update "Invalid Actions run id."
[[ "$run_number" =~ ^[0-9]+$ ]] || abort_update "Invalid Actions run number."
[[ "$version" == "build-$run_number" ]] || abort_update "Version does not match the Actions run number."
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || abort_update "Invalid build commit."

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

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
DB_USER=${DB_USER:-ai_pai}
DB_NAME=${DB_NAME:-ai_pai}
PUBLIC_ORIGIN=${APP_PUBLIC_ORIGIN:-}
previous_web=$(docker inspect ai-pai --format '{{.Config.Image}}')
previous_admin=$(docker inspect ai-pai-admin --format '{{.Config.Image}}')
previous_api=$(docker inspect ai-pai-api --format '{{.Config.Image}}')

timestamp=$(date +%Y%m%d-%H%M%S)
backup_dir="$BACKUP_ROOT/$version-$timestamp"
mkdir -p "$backup_dir"
cp -a "$COMPOSE_FILE" "$backup_dir/docker-compose.before.yml"
cp -a "$ENV_FILE" "$backup_dir/env.before"
docker inspect ai-pai ai-pai-admin ai-pai-api ai-pai-postgres > "$backup_dir/containers.before.json"

write_status backing_up "Creating and validating a PostgreSQL backup before the update."
docker exec ai-pai-postgres pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$backup_dir/ai_pai.dump"
docker exec -i ai-pai-postgres pg_restore -l < "$backup_dir/ai_pai.dump" > "$backup_dir/ai_pai.dump.list"
sha256sum "$backup_dir/ai_pai.dump" "$backup_dir/docker-compose.before.yml" "$backup_dir/env.before" > "$backup_dir/SHA256SUMS"

rollback_needed=true
set_env_value WEB_IMAGE "$web_image"
set_env_value ADMIN_IMAGE "$admin_image"
set_env_value API_IMAGE "$api_image"
export WEB_IMAGE="$web_image"
export ADMIN_IMAGE="$admin_image"
export API_IMAGE="$api_image"

write_status updating "Backup completed. Replacing application containers with $version."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --no-build \
  --force-recreate --wait --wait-timeout 240 api admin ai-pai

if [[ -n "$PUBLIC_ORIGIN" ]]; then
  public_base=${PUBLIC_ORIGIN%/}
  curl -fsS --retry 12 --retry-delay 5 "$public_base/healthz" >/dev/null
  curl -fsS --retry 12 --retry-delay 5 "$public_base/sys-admins/healthz" >/dev/null
fi

rollback_needed=false
trap - ERR
docker inspect ai-pai ai-pai-admin ai-pai-api ai-pai-postgres > "$backup_dir/containers.after.json"
write_status success "$version was deployed successfully." "$(date --iso-8601=seconds)"

mapfile -t old_backups < <(
  find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name 'build-*' -printf '%T@ %p\n' \
    | sort -nr | cut -d' ' -f2-
)
for ((index = BACKUP_RETENTION; index < ${#old_backups[@]}; index++)); do
  old_backup=${old_backups[$index]}
  if [[ "$old_backup" == "$BACKUP_ROOT"/build-* ]]; then
    rm -rf -- "$old_backup"
  fi
done

log "$version deployed successfully (previous version: $current_version)"
