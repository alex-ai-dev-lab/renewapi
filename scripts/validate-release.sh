#!/usr/bin/env bash
set -euo pipefail

prepare=false
if [[ "${1:-}" == "--prepare" ]]; then
  prepare=true
  shift
fi

tag="${1:-${GITHUB_REF_NAME:-}}"
expected_sha="${2:-${GITHUB_SHA:-}}"

if [[ -z "$tag" ]]; then
  echo "product release tag is required (for example: renewapi-v1.0.0-rc.1)" >&2
  exit 2
fi
if [[ ! "$tag" =~ ^renewapi-v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid product release tag: $tag" >&2
  exit 2
fi

product_version="${tag#renewapi-}"
version="$(tr -d '\r\n' < VERSION)"
if [[ "$version" != "$product_version" ]]; then
  echo "VERSION ($version) does not match product tag version ($product_version)" >&2
  exit 1
fi

if [[ "$prepare" == "true" ]]; then
  if git show-ref --tags --verify --quiet "refs/tags/$tag"; then
    echo "product release tag already exists: $tag" >&2
    exit 1
  fi
  if [[ "$product_version" == *-* ]]; then
    echo "product release preparation validation passed: prerelease $tag, VERSION=$product_version"
  else
    echo "product release preparation validation passed: stable $tag, VERSION=$product_version"
  fi
  exit 0
fi

if ! git show-ref --tags --verify --quiet "refs/tags/$tag"; then
  echo "tag is not available in the checkout: $tag" >&2
  exit 1
fi

tag_commit="$(git rev-list -n 1 "$tag")"
head_commit="$(git rev-parse HEAD)"
if [[ -n "$expected_sha" && "$tag_commit" != "$expected_sha" ]]; then
  echo "tag commit ($tag_commit) does not match expected source SHA ($expected_sha)" >&2
  exit 1
fi
if [[ "$tag_commit" != "$head_commit" ]]; then
  echo "tag commit ($tag_commit) does not match checked-out HEAD ($head_commit)" >&2
  exit 1
fi

tagged_version="$(git show "$tag:VERSION" | tr -d '\r\n')"
if [[ "$tagged_version" != "$product_version" ]]; then
  echo "tagged VERSION ($tagged_version) does not match product tag version ($product_version)" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is not clean" >&2
  git status --short >&2
  exit 1
fi

if [[ "$product_version" == *-* ]]; then
  echo "product release validation passed: prerelease $tag ($tag_commit), VERSION=$product_version"
else
  echo "product release validation passed: stable $tag ($tag_commit), VERSION=$product_version"
fi
