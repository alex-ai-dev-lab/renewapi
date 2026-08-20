#!/usr/bin/env bash

extract_compaction_item() {
  local response_file="$1"
  local item_file="$2"
  jq -e -c '
    first(
      ((if (.item | type) == "object" then [.item] else [] end) + (.output // .response.output // []))[]
      | select((.type == "compaction" or .type == "context_compaction" or .type == "compaction_summary") and (.encrypted_content | type == "string") and (.encrypted_content | length > 0))
    ) // empty
  ' "$response_file" >"$item_file"
}

extract_compaction_item_from_sse() {
  local response_file="$1"
  local item_file="$2"
  local line data data_file candidate_file
  local saw_frame=false
  local found_item=false

  : >"$item_file"
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ "$line" == data:* ]] || continue
    data="${line#data:}"
    data="${data# }"
    [[ -n "$data" ]] || continue
    [[ "$data" == '[DONE]' ]] && continue
    saw_frame=true
    if ! jq -e . <<<"$data" >/dev/null 2>&1; then
      SSE_EXTRACT_REASON=malformed_sse_json
      return 1
    fi
    data_file="$(mktemp)"
    candidate_file="$(mktemp)"
    printf '%s\n' "$data" >"$data_file"
    if extract_compaction_item "$data_file" "$candidate_file"; then
      mv -- "$candidate_file" "$item_file"
      found_item=true
    else
      rm -f -- "$candidate_file"
    fi
    rm -f -- "$data_file"
  done <"$response_file"

  if $found_item; then
    return 0
  fi
  if ! $saw_frame; then
    SSE_EXTRACT_REASON=missing_sse_data
  else
    SSE_EXTRACT_REASON=missing_compaction_item
  fi
  return 1
}
