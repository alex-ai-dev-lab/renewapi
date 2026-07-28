#!/bin/sh
set -eu

# Pattern aligned with relay/antipoison/protect.go sensitivePatterns
pattern='github_pat_|ghp_[A-Za-z0-9_]{20,}|gho_[A-Za-z0-9_]{20,}|ghu_[A-Za-z0-9_]{20,}|ghs_[A-Za-z0-9_]{20,}|ghr_[A-Za-z0-9_]{20,}|(^|[^A-Za-z0-9])sk-[A-Za-z0-9._-]{16,}|(^|[^A-Za-z0-9])sk-ant-[A-Za-z0-9._-]{16,}|(^|[^A-Za-z0-9])sk-proj-[A-Za-z0-9_-]{20,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z\-_]{35}|-----BEGIN [A-Z ]*PRIVATE KEY-----'

hits="$(git grep -nE "$pattern" -- . \
  ':!docs/**' \
  ':!.github/workflows/ci.yml' \
  ':!.github/workflows/security.yml' \
  ':!scripts/secret-scan.sh' \
  ':!relay/channel/vertex/service_account.go' \
  ':!relay/antipoison/guard_test.go' || true)"

if [ -n "$hits" ]; then
  printf '%s\n' "$hits" >&2
  exit 1
fi
