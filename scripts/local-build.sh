#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-dev}"
image="${NEWAPI_IMAGE:-ghcr.io/alex-ai-dev-lab/renewapi:$version}"
commit="$(git -C "$root" rev-parse --short=12 HEAD)"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
upstream="${UPSTREAM_REF:-4e570389dd433a717373ce9c9b822b59f5ed3d5d}"
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
