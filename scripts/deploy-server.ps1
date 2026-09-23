param(
  [string]$Image = $env:NEWAPI_IMAGE,
  [ValidateSet('local','pull','tar')]
  [string]$Mode = 'local',
  [switch]$DryRun,
  [switch]$AllowPasswordAuth
)

$ErrorActionPreference = 'Stop'
if ($Mode -eq 'pull') { throw '镜像改由 Releases 分发，请先在服务器 docker load，再使用 local 模式。' }
. (Join-Path $PSScriptRoot 'load-local-secrets.ps1') -Kind server

function Require-Env([string]$Name) {
  $value = [Environment]::GetEnvironmentVariable($Name, 'Process')
  if (-not $value) { throw "$Name missing from server-access.env" }
  return $value
}

$hostName = Require-Env 'SERVER_HOST'
$port = if ($env:SERVER_PORT) { $env:SERVER_PORT } else { '22' }
$user = Require-Env 'SERVER_USER'
$deployDir = Require-Env 'SERVER_DEPLOY_DIR'
$composeFile = if ($env:SERVER_COMPOSE_FILE) { $env:SERVER_COMPOSE_FILE } else { 'compose.yaml' }
$service = if ($env:SERVER_SERVICE_NAME) { $env:SERVER_SERVICE_NAME } else { 'new-api' }
$health = if ($env:SERVER_HEALTHCHECK_URL) { $env:SERVER_HEALTHCHECK_URL } else { 'http://127.0.0.1:3002/healthz' }
$proxy = $env:SERVER_PROXY
if (-not $Image) { $Image = $env:NEWAPI_IMAGE }
if (-not $Image) { throw "Image tag missing. Pass -Image or set NEWAPI_IMAGE." }

$sshBase = @('-p', $port, '-o', 'StrictHostKeyChecking=accept-new')
$scpBase = @('-P', $port, '-o', 'StrictHostKeyChecking=accept-new')
if ($env:SSH_AUTH_METHOD -eq 'key' -or ($env:SSH_KEY_PATH -and $env:SSH_AUTH_METHOD -ne 'password')) {
  if (-not (Test-Path -LiteralPath $env:SSH_KEY_PATH)) { throw "SSH key not found: $($env:SSH_KEY_PATH)" }
  $sshBase += @('-i', $env:SSH_KEY_PATH)
  $scpBase += @('-i', $env:SSH_KEY_PATH)
} elseif ($env:SSH_AUTH_METHOD -eq 'password') {
  if (-not $AllowPasswordAuth) { throw "Password SSH requested. Re-run with -AllowPasswordAuth after accepting local password automation risk." }
  if (-not (Get-Command sshpass -ErrorAction SilentlyContinue)) { throw "sshpass not found. Use SSH key mode or install sshpass." }
}

function Invoke-Remote([string]$Script) {
  if ($DryRun) {
    Write-Host "DRY-RUN remote script prepared for $user@$hostName"
    return
  }
  if ($env:SSH_AUTH_METHOD -eq 'password') {
    $env:SSHPASS = $env:SERVER_PASSWORD
    $Script | & sshpass -e ssh @sshBase "$user@$hostName" "sh -s"
  } else {
    $Script | & ssh @sshBase "$user@$hostName" "sh -s"
  }
}

$remoteScript = @"
set -eu
cd "$deployDir"
service="$service"
image="$Image"
compose_file="$composeFile"
health_url="$health"
proxy="$proxy"
if [ -n "`$proxy" ]; then
  export HTTP_PROXY="`$proxy" HTTPS_PROXY="`$proxy" http_proxy="`$proxy" https_proxy="`$proxy"
fi
mkdir -p backups
stamp=`$(date +%Y%m%d%H%M%S)
[ -f "`$compose_file" ] && cp "`$compose_file" "backups/`$compose_file.`$stamp.bak" || true
[ -f .env ] && cp .env "backups/.env.`$stamp.bak" || true
[ -f data/new-api.db ] && cp data/new-api.db "backups/new-api.db.`$stamp.bak" || true
[ -d logs ] && tar -czf "backups/logs.`$stamp.tgz" logs 2>/dev/null || true
old_image=`$(docker compose -f "`$compose_file" images -q "`$service" 2>/dev/null | head -n1 || true)
[ -n "`$old_image" ] && printf '%s\n' "`$old_image" > backups/previous-image.txt || true
docker image inspect "`$image" >/dev/null
NEWAPI_IMAGE="`$image" docker compose -f "`$compose_file" up -d --pull never --no-deps "`$service"
for i in `$(seq 1 30); do
  if curl -fsS "`$health_url" >/dev/null; then
    docker compose -f "`$compose_file" ps "`$service"
    docker compose -f "`$compose_file" logs --tail=100 "`$service" | grep -Ei 'panic|failed to open log file' && exit 20 || true
    echo "Deployment healthcheck passed"
    exit 0
  fi
  sleep 2
done
echo "Deployment healthcheck failed; attempting rollback" >&2
if [ -s backups/previous-image.txt ]; then
  previous=`$(cat backups/previous-image.txt)
  NEWAPI_IMAGE="`$previous" docker compose -f "`$compose_file" up -d --no-deps "`$service" || true
fi
exit 1
"@

Invoke-Remote $remoteScript
