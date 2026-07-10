#!/usr/bin/env bash
set -euo pipefail

remote="${UPSTREAM_REMOTE:-upstream}"
branch="${UPSTREAM_BRANCH:-main}"
upstream_url="${UPSTREAM_REPOSITORY_URL:-https://github.com/QuantumNous/new-api.git}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ledger="$repo_root/UPSTREAM_PORTS.md"

if ! git remote get-url "$remote" >/dev/null 2>&1; then
  git remote add "$remote" "$upstream_url"
  echo "added upstream remote: $upstream_url"
fi
git fetch "$remote" "$branch:refs/remotes/$remote/$branch" --tags

target="$(git rev-parse "$remote/$branch")"
if base="$(git merge-base HEAD "$remote/$branch" 2>/dev/null)"; then
  has_merge_base=true
else
  has_merge_base=false
fi

echo "upstream target: $remote/$branch ($target)"
if $has_merge_base; then
  echo "lineage mode: shared-history"
  echo "merge base: $base"
  echo "upstream commits ahead: $(git rev-list --count "HEAD..$remote/$branch")"
else
  echo "lineage mode: unrelated-history/manual-port"
  echo "merge/rebase is unsafe because the repositories have no common ancestor."
fi

audited_ref=""
if [ -f "$ledger" ]; then
  audited_ref="$(sed -nE 's/^Audited-Upstream-Ref:[[:space:]]*([0-9a-fA-F]{40})[[:space:]]*$/\1/p' "$ledger" | head -n 1)"
fi
if [ -n "$audited_ref" ] && git cat-file -e "$audited_ref^{commit}" 2>/dev/null; then
  if git merge-base --is-ancestor "$audited_ref" "$target"; then
    pending="$(git rev-list --count "$audited_ref..$target")"
    echo "last audited upstream: $audited_ref"
    echo "commits pending audit: $pending"
    if [ "$pending" -gt 0 ]; then
      git log --oneline --no-merges "$audited_ref..$target" | sed -n '1,120p'
    fi
  else
    echo "warning: audited ref $audited_ref is not an ancestor of $target" >&2
  fi
else
  echo "warning: UPSTREAM_PORTS.md has no usable Audited-Upstream-Ref" >&2
fi

if $has_merge_base; then
  echo "fork diff since merge base:"
  git diff --stat "$base..HEAD" -- . ':!legacy/patches/**'
fi
