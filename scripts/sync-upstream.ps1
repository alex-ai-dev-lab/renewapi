param(
  [ValidateSet('port', 'merge', 'rebase')][string]$Mode = 'port',
  [switch]$DryRun
)
$ErrorActionPreference = 'Stop'

$remote = if ($env:UPSTREAM_REMOTE) { $env:UPSTREAM_REMOTE } else { 'upstream' }
$branch = if ($env:UPSTREAM_BRANCH) { $env:UPSTREAM_BRANCH } else { 'main' }
$upstreamUrl = if ($env:UPSTREAM_REPOSITORY_URL) { $env:UPSTREAM_REPOSITORY_URL } else { 'https://github.com/QuantumNous/new-api.git' }

git remote get-url $remote *> $null
if ($LASTEXITCODE -ne 0) { git remote add $remote $upstreamUrl }
git fetch $remote "$branch`:refs/remotes/$remote/$branch" --tags
if ($LASTEXITCODE -ne 0) { throw "failed to fetch $remote/$branch" }

$target = "$remote/$branch"
$base = git merge-base HEAD $target 2>$null
$hasMergeBase = $LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($base)

Write-Host "mode: $Mode"
Write-Host "target: $target ($(git rev-parse $target))"

if (-not $hasMergeBase) {
  if ($Mode -ne 'port') {
    throw "refusing ${Mode}: HEAD and $target have no common ancestor; use -Mode port and UPSTREAM_PORTS.md"
  }
  & (Join-Path $PSScriptRoot 'check-upstream.ps1')
  Write-Host 'port mode is audit-only; apply selected commits manually and record them in UPSTREAM_PORTS.md.'
  exit 0
}

if ($Mode -eq 'port') {
  & (Join-Path $PSScriptRoot 'check-upstream.ps1')
  Write-Host 'shared history detected; choose -Mode merge or -Mode rebase explicitly to mutate the branch.'
  exit 0
}

if ($DryRun) {
  git merge-tree $base HEAD $target *> $null
  if ($LASTEXITCODE -ne 0) { throw 'dry-run merge-tree failed' }
  Write-Host 'dry-run merge-tree completed'
  exit 0
}

if ($Mode -eq 'rebase') {
  git rebase $target
} else {
  git merge --no-ff $target
}
if ($LASTEXITCODE -ne 0) { throw "$Mode failed" }

if (Get-Command go -ErrorAction SilentlyContinue) {
  go test ./relay/antipoison ./service ./model ./controller
  if ($LASTEXITCODE -ne 0) { throw 'post-sync tests failed' }
} else {
  Write-Host 'go not found; skipped minimal tests'
}
