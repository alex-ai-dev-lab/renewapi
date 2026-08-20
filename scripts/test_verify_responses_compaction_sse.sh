#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture_dir="$script_dir/testdata/responses_compaction"
tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT
# shellcheck source=lib/responses_compaction_sse.sh
source "$script_dir/lib/responses_compaction_sse.sh"

assert_rejected() {
  local fixture="$1"
  local expected_reason="$2"
  local item="$tmp_dir/item.json"
  SSE_EXTRACT_REASON=""
  if extract_compaction_item_from_sse "$fixture_dir/$fixture" "$item"; then
    printf '%s unexpectedly passed\n' "$fixture" >&2
    exit 1
  fi
  if [[ "$SSE_EXTRACT_REASON" != "$expected_reason" ]]; then
    printf '%s reason=%s expected=%s\n' "$fixture" "$SSE_EXTRACT_REASON" "$expected_reason" >&2
    exit 1
  fi
}

item="$tmp_dir/valid-item.json"
extract_compaction_item_from_sse "$fixture_dir/valid_compaction_sse.txt" "$item"
jq -e '.type == "compaction_summary" and .encrypted_content == "opaque-stream"' "$item" >/dev/null
assert_rejected ordinary_sse.txt missing_compaction_item
assert_rejected missing_encrypted_content_sse.txt missing_compaction_item
assert_rejected malformed_sse.txt malformed_sse_json

printf 'responses_compaction_sse_fixtures=PASS\n'
