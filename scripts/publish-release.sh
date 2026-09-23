#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REPOSITORY:?}"
: "${GITHUB_SHA:?}"
: "${RELEASE_TAG:?}"
: "${RELEASE_NAME:?}"
: "${PRERELEASE:?}"
: "${MAKE_LATEST:?}"

assets='release-assets'
test -s "$assets/CHECKSUMS.txt"
(cd "$assets" && sha256sum -c CHECKSUMS.txt)

# 先查询，避免把鉴权或网络故障误当成不存在的 Release。
gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/releases?per_page=100" > "$RUNNER_TEMP/releases.json"
existing="$(jq --arg tag "$RELEASE_TAG" '[.[][] | select(.tag_name == $tag)][0] // null' "$RUNNER_TEMP/releases.json")"
# Git tag 是源码事实，已存在标签不得被重指向。
references="$(gh api "repos/$GITHUB_REPOSITORY/git/matching-refs/tags/$RELEASE_TAG")"
reference="$(jq --arg ref "refs/tags/$RELEASE_TAG" '[.[] | select(.ref == $ref)][0] // null' <<< "$references")"
if [[ "$reference" == 'null' ]]; then
  [[ "$RELEASE_TAG" == renewapi-build-* || "$RELEASE_TAG" == renewapi-source-* ]]
  reference="$(gh api --method POST "repos/$GITHUB_REPOSITORY/git/refs" -f ref="refs/tags/$RELEASE_TAG" -f sha="$GITHUB_SHA")"
fi
object_type="$(jq -r '.object.type' <<< "$reference")"
actual="$(jq -r '.object.sha' <<< "$reference")"
while [[ "$object_type" == 'tag' ]]; do
  reference="$(gh api "repos/$GITHUB_REPOSITORY/git/tags/$actual")"
  object_type="$(jq -r '.object.type' <<< "$reference")"
  actual="$(jq -r '.object.sha' <<< "$reference")"
done
test "$object_type" = 'commit'
test "$actual" = "$GITHUB_SHA"

if [[ "$existing" == 'null' ]]; then
  flags=(--draft --latest=false --verify-tag)
  if [[ "$PRERELEASE" == 'true' ]]; then flags+=(--prerelease); fi
  gh release create "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" \
    --title "$RELEASE_NAME" --notes-file "$assets/RELEASE_NOTES.md" "${flags[@]}"
fi

verify_download() {
  local destination
  destination="$(mktemp -d "$RUNNER_TEMP/renewapi-release-verify.XXXXXX")"
  gh release download "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --dir "$destination"
  test -s "$destination/CHECKSUMS.txt"
  test "$(find "$destination" -maxdepth 1 -name '*-linux-*.tar.gz' | wc -l)" -eq 2
  (cd "$destination" && sha256sum -c CHECKSUMS.txt)
}

if [[ "$existing" != 'null' && "$(jq -r '.draft' <<< "$existing")" == 'false' ]]; then
  # 成功发布后的重跑只验证原产物，不覆盖历史附件。
  verify_download
  printf '已核验既有 Release：%s\n' "$RELEASE_TAG" >> "$GITHUB_STEP_SUMMARY"
  exit 0
fi

gh release upload "$RELEASE_TAG" "$assets"/* --repo "$GITHUB_REPOSITORY" --clobber
verify_download
gh release edit "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --draft=false \
  --prerelease="$PRERELEASE" --latest="$MAKE_LATEST" \
  --title "$RELEASE_NAME" --notes-file "$assets/RELEASE_NOTES.md"
printf 'Release 已发布：https://github.com/%s/releases/tag/%s\n' "$GITHUB_REPOSITORY" "$RELEASE_TAG" >> "$GITHUB_STEP_SUMMARY"
