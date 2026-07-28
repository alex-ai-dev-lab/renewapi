$ErrorActionPreference = 'Stop'

$remote = if ($env:UPSTREAM_REMOTE) { $env:UPSTREAM_REMOTE } else { 'upstream' }
$branch = if ($env:UPSTREAM_BRANCH) { $env:UPSTREAM_BRANCH } else { 'main' }
$upstreamUrl = if ($env:UPSTREAM_REPOSITORY_URL) { $env:UPSTREAM_REPOSITORY_URL } else { 'https://github.com/QuantumNous/new-api.git' }
$ledgerPath = Join-Path $PSScriptRoot '..\UPSTREAM_PORTS.md'

git remote get-url $remote *> $null
if ($LASTEXITCODE -ne 0) {
  git remote add $remote $upstreamUrl
  Write-Host "added upstream remote: $upstreamUrl"
}

git fetch $remote "$branch`:refs/remotes/$remote/$branch" --tags
if ($LASTEXITCODE -ne 0) { throw "failed to fetch $remote/$branch" }

$target = (git rev-parse "$remote/$branch").Trim()
$base = git merge-base HEAD "$remote/$branch" 2>$null
$hasMergeBase = $LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($base)

Write-Host "upstream target: $remote/$branch ($target)"
Write-Host "lineage mode: $(if ($hasMergeBase) { 'shared-history' } else { 'unrelated-history/manual-port' })"
if ($hasMergeBase) {
  Write-Host "merge base: $($base.Trim())"
  Write-Host "upstream commits ahead: $(git rev-list --count "HEAD..$remote/$branch")"
} else {
  Write-Host 'merge/rebase is unsafe because the repositories have no common ancestor.'
}

$auditedRef = $null
if (Test-Path $ledgerPath) {
  $match = Select-String -Path $ledgerPath -Pattern '^Audited-Upstream-Ref:\s*([0-9a-fA-F]{40})\s*$' | Select-Object -First 1
  if ($match) { $auditedRef = $match.Matches[0].Groups[1].Value }
}

$hasAuditedCommit = $false
if ($auditedRef) {
  git cat-file -e "$auditedRef`^{commit}" 2>$null
  $hasAuditedCommit = $LASTEXITCODE -eq 0
}
if ($auditedRef -and $hasAuditedCommit) {
  git merge-base --is-ancestor $auditedRef $target 2>$null
  if ($LASTEXITCODE -eq 0) {
    $pending = (git rev-list --count "$auditedRef..$target").Trim()
    Write-Host "last audited upstream: $auditedRef"
    Write-Host "commits pending audit: $pending"
    if ([int]$pending -gt 0) {
      git log --oneline --no-merges "$auditedRef..$target" | Select-Object -First 120
    }
  } else {
    Write-Warning "audited ref $auditedRef is not an ancestor of $target; inspect upstream history rewrite manually"
  }
} else {
  Write-Warning 'UPSTREAM_PORTS.md has no usable Audited-Upstream-Ref; a full manual audit is required.'
}

if ($hasMergeBase) {
  Write-Host 'fork diff since merge base:'
  git diff --stat "$($base.Trim())..HEAD" -- .
}
