#!/usr/bin/env bash
set -euo pipefail

mode="port"
dry_run=false
while [ $# -gt 0 ]; do
  case "$1" in
    --port) mode="port"; shift ;;
    --merge) mode="merge"; shift ;;
    --rebase) mode="rebase"; shift ;;
    --dry-run) dry_run=true; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 2 ;;
  esac
done

remote="${UPSTREAM_REMOTE:-upstream}"
branch="${UPSTREAM_BRANCH:-main}"
upstream_url="${UPSTREAM_REPOSITORY_URL:-https://github.com/QuantumNous/new-api.git}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

git remote get-url "$remote" >/dev/null 2>&1 || git remote add "$remote" "$upstream_url"
git fetch "$remote" "$branch:refs/remotes/$remote/$branch" --tags
target="$remote/$branch"
if base="$(git merge-base HEAD "$target" 2>/dev/null)"; then
  has_merge_base=true
else
  has_merge_base=false
fi

echo "mode: $mode"
echo "target: $target ($(git rev-parse "$target"))"

if ! $has_merge_base; then
  if [ "$mode" != "port" ]; then
    echo "refusing $mode: HEAD and $target have no common ancestor; use --port and UPSTREAM_PORTS.md" >&2
    exit 1
  fi
  "$script_dir/check-upstream.sh"
  echo "port mode is audit-only; apply selected commits manually and record them in UPSTREAM_PORTS.md."
  exit 0
fi

if [ "$mode" = "port" ]; then
  "$script_dir/check-upstream.sh"
  echo "shared history detected; choose --merge or --rebase explicitly to mutate the branch."
  exit 0
fi

if $dry_run; then
  git merge-tree "$base" HEAD "$target" >/dev/null
  echo "dry-run merge-tree completed"
  exit 0
fi

if [ "$mode" = "rebase" ]; then
  git rebase "$target"
else
  git merge --no-ff "$target"
fi

if command -v go >/dev/null 2>&1; then
  go test ./relay/antipoison ./service ./model ./controller
else
  echo "go not found; skipped minimal tests"
fi
