#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-$(tr -d '\r\n' < "$root/VERSION")}"
build_channel="${BUILD_CHANNEL:-local}"
docker_version="${version#v}"
image="${NEWAPI_IMAGE:-ghcr.io/alex-ai-dev-lab/renewapi:$docker_version}"
commit="$(git -C "$root" rev-parse --short=12 HEAD)"
date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
upstream="${UPSTREAM_REF:-58d4e9bd3bb035df8ea235dd682ccc8a45d0332a}"
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
  --build-arg "BUILD_CHANNEL=$build_channel" \
  --build-arg "UPSTREAM_REF=$upstream" \
  -t "$image" \
  "${extra[@]}" \
  "$root"
