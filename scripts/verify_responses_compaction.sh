#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/responses_compaction_sse.sh
source "$script_dir/lib/responses_compaction_sse.sh"

: "${BASE_URL:?BASE_URL is required}"
: "${API_KEY:?API_KEY is required}"
MODEL="${MODEL:-gpt-5.6-sol}"

base_url="${BASE_URL%/}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT

curl_config="$tmp_dir/curl.conf"
umask 077
{
  printf 'silent\nshow-error\n'
  printf 'connect-timeout = "%s"\n' "${CONNECT_TIMEOUT_SECONDS:-15}"
  printf 'max-time = "%s"\n' "${REQUEST_TIMEOUT_SECONDS:-180}"
  printf 'header = "Authorization: Bearer %s"\n' "$API_KEY"
  printf 'header = "Content-Type: application/json"\n'
} >"$curl_config"

post_json() {
  local path="$1"
  local body_file="$2"
  local output_file="$3"
  local http_status

  if http_status="$(curl --config "$curl_config" \
    --output "$output_file" \
    --write-out '%{http_code}' \
    --data-binary "@${body_file}" \
    "${base_url}${path}")"; then
    :
  else
    LAST_HTTP_STATUS="${http_status:-000}"
    LAST_RESULT=FAIL
    return 1
  fi

  LAST_HTTP_STATUS="$http_status"
  case "$http_status" in
    2??)
      LAST_RESULT=PASS
      return 0
      ;;
    404|405|501)
      LAST_RESULT=UNSUPPORTED
      return 2
      ;;
    401|403)
      LAST_RESULT=FAIL_AUTH_CONFIG
      return 3
      ;;
    429|500|502|503|504)
      LAST_RESULT=FAIL_TRANSIENT
      return 4
      ;;
    400|422)
      LAST_RESULT=FAIL_INVALID_PROBE
      return 5
      ;;
    *)
      LAST_RESULT=FAIL
      return 1
      ;;
  esac
}

report_request() {
  local facet="$1"
  local path="$2"
  local body_file="$3"
  local output_file="$4"

  if post_json "$path" "$body_file" "$output_file"; then
    printf '%s=PASS status=%s\n' "$facet" "$LAST_HTTP_STATUS"
    return 0
  else
    local request_result=$?
  fi

  printf '%s=%s status=%s\n' "$facet" "$LAST_RESULT" "$LAST_HTTP_STATUS"
  return "$request_result"
}

report_sse_request() {
  local facet="$1"
  local path="$2"
  local body_file="$3"
  local output_file="$4"
  local headers_file="$5"
  local http_status

  if http_status="$(curl --config "$curl_config" \
    --dump-header "$headers_file" \
    --output "$output_file" \
    --write-out '%{http_code}' \
    --data-binary "@${body_file}" \
    "${base_url}${path}")"; then
    :
  else
    LAST_HTTP_STATUS="${http_status:-000}"
    LAST_RESULT=FAIL
    return 1
  fi

  LAST_HTTP_STATUS="$http_status"
  case "$http_status" in
    2??)
      LAST_RESULT=PASS
      return 0
      ;;
    404|405|501)
      LAST_RESULT=UNSUPPORTED
      return 2
      ;;
    401|403)
      LAST_RESULT=FAIL_AUTH_CONFIG
      return 3
      ;;
    429|500|502|503|504)
      LAST_RESULT=FAIL_TRANSIENT
      return 4
      ;;
    400|422)
      LAST_RESULT=FAIL_INVALID_PROBE
      return 5
      ;;
    *)
      LAST_RESULT=FAIL
      return 1
      ;;
  esac
}

normal_body="$tmp_dir/normal-request.json"
normal_response="$tmp_dir/normal-response.json"
jq -n --arg model "$MODEL" '{model:$model,input:"Reply with NORMAL_OK.",store:false,stream:false}' >"$normal_body"
if ! report_request ordinary_responses '/v1/responses' "$normal_body" "$normal_response"; then
  exit 1
fi

legacy_body="$tmp_dir/legacy-request.json"
legacy_response="$tmp_dir/legacy-response.json"
jq -n --arg model "$MODEL" '{model:$model,input:[{role:"user",content:[{type:"input_text",text:"Compress this synthetic verification state."}]}]}' >"$legacy_body"
legacy_ok=false
if report_request legacy_compact '/v1/responses/compact' "$legacy_body" "$legacy_response"; then
  legacy_ok=true
fi

native_body="$tmp_dir/native-request.json"
native_response="$tmp_dir/native-response.json"
jq -n --arg model "$MODEL" '{model:$model,input:[{role:"user",content:[{type:"input_text",text:"Compress this synthetic verification state."}]},{type:"compaction_trigger"}],store:false,stream:false}' >"$native_body"
native_ok=false
if report_request native_compact '/v1/responses' "$native_body" "$native_response"; then
  native_ok=true
fi

stream_body="$tmp_dir/stream-request.json"
stream_response="$tmp_dir/stream-response.txt"
stream_headers="$tmp_dir/stream-headers.txt"
jq -n --arg model "$MODEL" '{model:$model,input:[{role:"user",content:[{type:"input_text",text:"Compress this synthetic verification state."}]},{type:"compaction_trigger"}],store:false,stream:true}' >"$stream_body"
stream_ok=false
stream_item="$tmp_dir/stream-compaction-item.json"
if report_sse_request native_sse_http '/v1/responses' "$stream_body" "$stream_response" "$stream_headers"; then
  content_type="$(awk 'BEGIN{IGNORECASE=1} /^Content-Type:/ {sub(/^[^:]*:[[:space:]]*/, ""); value=$0} END{print value}' "$stream_headers" | tr -d '\r' | tr '[:upper:]' '[:lower:]')"
  if [[ "$content_type" != text/event-stream* ]]; then
    printf 'native_sse=FAIL invalid_content_type content_type=%s\n' "${content_type:-missing}"
  elif extract_compaction_item_from_sse "$stream_response" "$stream_item"; then
    stream_ok=true
    printf 'native_sse=PASS\n'
  else
    printf 'native_sse=FAIL %s\n' "$SSE_EXTRACT_REASON"
  fi
else
  printf 'native_sse=SKIP http_request_failed\n'
fi

compaction_item="$tmp_dir/compaction-item.json"
if $native_ok && extract_compaction_item "$native_response" "$compaction_item"; then
  :
elif $legacy_ok && extract_compaction_item "$legacy_response" "$compaction_item"; then
  :
else
  printf 'continuation=SKIP no_compaction_item\n'
  printf 'repeated_compact=SKIP no_compaction_item\n'
  if ! $legacy_ok && ! $native_ok; then
    exit 2
  fi
  exit 1
fi

continuation_body="$tmp_dir/continuation-request.json"
continuation_response="$tmp_dir/continuation-response.json"
jq -n --arg model "$MODEL" --slurpfile item "$compaction_item" '{model:$model,input:[$item[0],{role:"user",content:[{type:"input_text",text:"Reply with CONTINUE_OK."}]}],store:false,stream:false}' >"$continuation_body"
if ! report_request continuation '/v1/responses' "$continuation_body" "$continuation_response"; then
  exit 1
fi

repeat_body="$tmp_dir/repeat-request.json"
repeat_response="$tmp_dir/repeat-response.json"
jq -n --arg model "$MODEL" --slurpfile item "$compaction_item" '{model:$model,input:[$item[0],{role:"user",content:[{type:"input_text",text:"Compact the continued synthetic state again."}]},{type:"compaction_trigger"}],store:false,stream:false}' >"$repeat_body"
if ! report_request repeated_compact '/v1/responses' "$repeat_body" "$repeat_response"; then
  exit 1
fi
if ! extract_compaction_item "$repeat_response" "$compaction_item"; then
  printf 'repeated_compaction_item=FAIL missing_compaction_item\n'
  exit 1
fi
jq -n --arg model "$MODEL" --slurpfile item "$compaction_item" '{model:$model,input:[$item[0],{role:"user",content:[{type:"input_text",text:"Reply with CONTINUE_AGAIN_OK."}]}],store:false,stream:false}' >"$continuation_body"
if ! report_request repeated_continuation '/v1/responses' "$continuation_body" "$continuation_response"; then
  exit 1
fi

if ! $stream_ok; then
  exit 2
fi

printf 'repeated_compact_and_continue=PASS\n'
