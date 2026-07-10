param(
  [string]$Image = $env:NEWAPI_IMAGE,
  [string]$Version = $(if ($env:VERSION) { $env:VERSION } else { "dev" }),
  [switch]$Push,
  [switch]$Load
)

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (-not $Image) { $Image = "ghcr.io/alex-ai-dev-lab/renewapi:$Version" }
$commit = (git -C $root rev-parse --short=12 HEAD)
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$upstream = if ($env:UPSTREAM_REF) { $env:UPSTREAM_REF } else { '4e570389dd433a717373ce9c9b822b59f5ed3d5d' }
$platforms = if ($Load) { "linux/amd64" } else { "linux/amd64,linux/arm64" }
$args = @(
  "buildx","build",
  "--platform",$platforms,
  "--build-arg","VERSION=$Version",
  "--build-arg","COMMIT_SHA=$commit",
  "--build-arg","BUILD_DATE=$date",
  "--build-arg","UPSTREAM_REF=$upstream",
  "-t",$Image
)
if ($Push) { $args += "--push" } elseif ($Load) { $args += "--load" }
$args += $root
docker @args
