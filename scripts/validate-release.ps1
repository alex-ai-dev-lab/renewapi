param(
  [string]$Tag = $env:GITHUB_REF_NAME,
  [string]$ExpectedSha = $env:GITHUB_SHA,
  [switch]$Prepare
)

$ErrorActionPreference = 'Stop'

if (-not $Tag) { throw 'Product release tag is required (for example: renewapi-v1.0.0-rc.1).' }
if ($Tag -notmatch '^renewapi-v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') { throw "Invalid product release tag: $Tag" }

$productVersion = $Tag -replace '^renewapi-', ''
$version = (Get-Content -Raw -LiteralPath 'VERSION').Trim()
if ($version -ne $productVersion) { throw "VERSION ($version) does not match product tag version ($productVersion)" }

$ref = "refs/tags/$Tag"
if ($Prepare) {
  git show-ref --tags --verify --quiet $ref
  if ($LASTEXITCODE -eq 0) { throw "Product release tag already exists: $Tag" }
  $kind = if ($productVersion.Contains('-')) { 'prerelease' } else { 'stable' }
  Write-Host "Product release preparation validation passed: $kind $Tag, VERSION=$productVersion"
  return
}

git show-ref --tags --verify --quiet $ref
if ($LASTEXITCODE -ne 0) { throw "Tag is not available in the checkout: $Tag" }

$tagCommit = (git rev-list -n 1 $Tag).Trim()
$headCommit = (git rev-parse HEAD).Trim()
if ($ExpectedSha -and $tagCommit -ne $ExpectedSha) { throw "Tag commit ($tagCommit) does not match expected source SHA ($ExpectedSha)" }
if ($tagCommit -ne $headCommit) { throw "Tag commit ($tagCommit) does not match checked-out HEAD ($headCommit)" }

$taggedVersion = (git show "$Tag`:VERSION").Trim()
if ($taggedVersion -ne $productVersion) { throw "Tagged VERSION ($taggedVersion) does not match product tag version ($productVersion)" }

if ((git status --porcelain)) {
  git status --short
  throw 'Working tree is not clean'
}

$kind = if ($productVersion.Contains('-')) { 'prerelease' } else { 'stable' }
Write-Host "Product release validation passed: $kind $Tag ($tagCommit), VERSION=$productVersion"
