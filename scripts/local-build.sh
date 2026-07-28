#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-dev}"
image="${NEWAPI_IMAGE:-ghcr.io/alex-ai-dev-lab/renewapi:$version}"
commit="$(git -C "$root" rev-parse --short=12 HEAD)"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
upstream="${UPSTREAM_REF:-c3db41407dd1a0662ef630c41de4ac0c48c83e3c}"
platforms="${PLATFORMS:-linux/amd64,linux/arm64}"
extra=()
case "${1:-}" in
  --push) extra+=(--push) ;;
  --load) platforms="linux/amd64"; extra+=(--load) ;;
esac

docker buildx build \
  --platform "$platforms" \
  --build-arg "VERSION=$version" \
  --build-arg "COMMIT_SHA=$commit" \
  --build-arg "BUILD_DATE=$date" \
  --build-arg "UPSTREAM_REF=$upstream" \
  -t "$image" \
  "${extra[@]}" \
  "$root"
