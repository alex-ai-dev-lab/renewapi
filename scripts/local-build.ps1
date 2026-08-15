param(
  [string]$Image = $env:NEWAPI_IMAGE,
  [string]$Version = $env:VERSION,
  [string]$BuildChannel = $(if ($env:BUILD_CHANNEL) { $env:BUILD_CHANNEL } else { "local" }),
  [switch]$Push,
  [switch]$Load
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $Version) { $Version = (Get-Content -Raw -LiteralPath (Join-Path $root 'VERSION')).Trim() }
$dockerVersion = $Version -replace '^v', ''
if (-not $Image) { $Image = "ghcr.io/alex-ai-dev-lab/renewapi:$dockerVersion" }
$commit = (git -C $root rev-parse --short=12 HEAD)
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$upstream = if ($env:UPSTREAM_REF) { $env:UPSTREAM_REF } else { '58d4e9bd3bb035df8ea235dd682ccc8a45d0332a' }
$platforms = if ($Load) { "linux/amd64" } else { "linux/amd64,linux/arm64" }
$args = @(
  "buildx","build",
  "--platform",$platforms,
  "--build-arg","VERSION=$Version",
  "--build-arg","COMMIT_SHA=$commit",
  "--build-arg","BUILD_DATE=$date",
  "--build-arg","BUILD_CHANNEL=$BuildChannel",
  "--build-arg","UPSTREAM_REF=$upstream",
  "-t",$Image
)
if ($Push) { $args += "--push" } elseif ($Load) { $args += "--load" }
$args += $root
docker @args
