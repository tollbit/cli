#!/usr/bin/env bash
# Push release-facing files to tollbit/tollbit-cli-releases (branch main), then mirror the
# release git tag onto main's HEAD. GoReleaser publishes to that repo and GitHub requires the
# tag to exist there (otherwise uploads fail with "Repository is empty" / untagged releases).
#
# Requires RELEASES_GITHUB_TOKEN with Contents: Read and write on that repo.
set -euo pipefail

TOKEN="${RELEASES_GITHUB_TOKEN:?RELEASES_GITHUB_TOKEN is not set}"
TAG="${GITHUB_REF_NAME:?GITHUB_REF_NAME is not set}"
SRC="${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is not set}"
REMOTE_URL="https://x-access-token:${TOKEN}@github.com/tollbit/tollbit-cli-releases.git"
TARGET_BRANCH="main"

TMP="$(mktemp -d)"
cleanup() { rm -rf "${TMP}"; }
trap cleanup EXIT

git clone "${REMOTE_URL}" "${TMP}/pub"
cd "${TMP}/pub"

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
  git checkout -b "${TARGET_BRANCH}" 2>/dev/null || true
fi

mkdir -p scripts
install -m0755 "${SRC}/scripts/install.sh" scripts/install.sh
install -m0644 "${SRC}/scripts/install.ps1" scripts/install.ps1
install -m0644 "${SRC}/release-public/README.md" README.md

git add scripts/install.sh scripts/install.ps1 README.md

have_head() {
  git rev-parse --verify HEAD >/dev/null 2>&1
}

if have_head && git diff --cached --quiet; then
  echo "No changes to synced release files."
elif have_head; then
  git commit -m "chore: sync release artifacts from tollbit-cli ${TAG}"
  git push origin "HEAD:refs/heads/${TARGET_BRANCH}"
else
  git commit -m "chore: sync release artifacts from tollbit-cli ${TAG}"
  git branch -M "${TARGET_BRANCH}" 2>/dev/null || true
  git push -u origin "${TARGET_BRANCH}"
fi

git fetch origin
git checkout "${TARGET_BRANCH}"
git reset --hard "origin/${TARGET_BRANCH}"

HEAD_SHA="$(git rev-parse HEAD)"

mirror_tag() {
  local remote_sha=""
  if git ls-remote --tags origin "refs/tags/${TAG}" | grep -q .; then
    remote_sha="$(git ls-remote origin "refs/tags/${TAG}" | awk '{print $1}' | head -n1)"
  fi
  if [[ -n "${remote_sha}" && "${remote_sha}" == "${HEAD_SHA}" ]]; then
    echo "Tag ${TAG} already on tollbit-cli-releases at ${HEAD_SHA}"
    return 0
  fi
  if [[ -n "${remote_sha}" ]]; then
    echo "Replacing remote tag ${TAG} (${remote_sha} -> ${HEAD_SHA})"
    git push origin ":refs/tags/${TAG}"
  fi
  git tag "${TAG}" "${HEAD_SHA}"
  git push origin "${TAG}"
}

mirror_tag

echo "Synced tollbit-cli-releases ${TARGET_BRANCH} and mirrored tag ${TAG} at ${HEAD_SHA}."
