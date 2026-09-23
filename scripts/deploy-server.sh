#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mode="local"
dry_run="false"
allow_password="false"
image="${NEWAPI_IMAGE:-}"
while [ $# -gt 0 ]; do
  case "$1" in
    --image) image="$2"; shift 2 ;;
    --mode) mode="$2"; shift 2 ;;
    --dry-run) dry_run="true"; shift ;;
    --allow-password-auth) allow_password="true"; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 2 ;;
  esac
done

case "$mode" in
  local|tar) ;;
  *) echo '镜像改由 Releases 分发，请先在服务器 docker load，再使用 local 模式。' >&2; exit 2 ;;
esac

# shellcheck source=/dev/null
source "$root/scripts/load-local-secrets.sh" server >/dev/null
: "${SERVER_HOST:?SERVER_HOST missing}"
: "${SERVER_USER:?SERVER_USER missing}"
: "${SERVER_DEPLOY_DIR:?SERVER_DEPLOY_DIR missing}"
: "${image:?image missing, pass --image or set NEWAPI_IMAGE}"

SERVER_PORT="${SERVER_PORT:-22}"
SERVER_COMPOSE_FILE="${SERVER_COMPOSE_FILE:-compose.yaml}"
SERVER_SERVICE_NAME="${SERVER_SERVICE_NAME:-new-api}"
SERVER_HEALTHCHECK_URL="${SERVER_HEALTHCHECK_URL:-http://127.0.0.1:3002/healthz}"
SERVER_PROXY="${SERVER_PROXY:-}"

ssh_args=(-p "$SERVER_PORT" -o StrictHostKeyChecking=accept-new)
scp_args=(-P "$SERVER_PORT" -o StrictHostKeyChecking=accept-new)
if [ "${SSH_AUTH_METHOD:-key}" = "password" ]; then
  [ "$allow_password" = "true" ] || { echo "Password SSH requested. Re-run with --allow-password-auth after accepting local password automation risk." >&2; exit 2; }
  command -v sshpass >/dev/null || { echo "sshpass not found. Use SSH key mode or install sshpass." >&2; exit 2; }
  export SSHPASS="${SERVER_PASSWORD:-}"
else
  if [ -n "${SSH_KEY_PATH:-}" ]; then
    ssh_args+=(-i "$SSH_KEY_PATH")
    scp_args+=(-i "$SSH_KEY_PATH")
  fi
fi

remote_script=$(cat <<REMOTE
set -eu
cd "$SERVER_DEPLOY_DIR"
service="$SERVER_SERVICE_NAME"
image="$image"
compose_file="$SERVER_COMPOSE_FILE"
health_url="$SERVER_HEALTHCHECK_URL"
proxy="$SERVER_PROXY"
if [ -n "\$proxy" ]; then
  export HTTP_PROXY="\$proxy" HTTPS_PROXY="\$proxy" http_proxy="\$proxy" https_proxy="\$proxy"
fi
mkdir -p backups
stamp=\$(date +%Y%m%d%H%M%S)
[ -f "\$compose_file" ] && cp "\$compose_file" "backups/\$compose_file.\$stamp.bak" || true
[ -f .env ] && cp .env "backups/.env.\$stamp.bak" || true
[ -f data/new-api.db ] && cp data/new-api.db "backups/new-api.db.\$stamp.bak" || true
[ -d logs ] && tar -czf "backups/logs.\$stamp.tgz" logs 2>/dev/null || true
old_image=\$(docker compose -f "\$compose_file" images -q "\$service" 2>/dev/null | head -n1 || true)
[ -n "\$old_image" ] && printf '%s\n' "\$old_image" > backups/previous-image.txt || true
docker image inspect "\$image" >/dev/null
NEWAPI_IMAGE="\$image" docker compose -f "\$compose_file" up -d --pull never --no-deps "\$service"
for i in \$(seq 1 30); do
  if curl -fsS "\$health_url" >/dev/null; then
    docker compose -f "\$compose_file" ps "\$service"
    docker compose -f "\$compose_file" logs --tail=100 "\$service" | grep -Ei 'panic|failed to open log file' && exit 20 || true
    echo "Deployment healthcheck passed"
    exit 0
  fi
  sleep 2
done
echo "Deployment healthcheck failed; attempting rollback" >&2
if [ -s backups/previous-image.txt ]; then
  previous=\$(cat backups/previous-image.txt)
  NEWAPI_IMAGE="\$previous" docker compose -f "\$compose_file" up -d --no-deps "\$service" || true
fi
exit 1
REMOTE
)

if [ "$dry_run" = "true" ]; then
  echo "DRY-RUN remote script prepared for $SERVER_USER@$SERVER_HOST"
  exit 0
fi
if [ "${SSH_AUTH_METHOD:-key}" = "password" ]; then
  sshpass -e ssh "${ssh_args[@]}" "$SERVER_USER@$SERVER_HOST" "sh -s" <<< "$remote_script"
else
  ssh "${ssh_args[@]}" "$SERVER_USER@$SERVER_HOST" "sh -s" <<< "$remote_script"
fi
