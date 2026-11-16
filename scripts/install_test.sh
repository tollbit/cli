#!/usr/bin/env bash
set -euo pipefail

# Tests install.sh with a fake curl on PATH that serves two fixture folders:
# one for the tollbit.com mirror (https://mirror.test/.agents/cli/...) and one
# for GitHub Releases and its latest-release API. The release holds a fake
# tollbit binary for this machine's OS and architecture. Each test:
#   1. Builds both folders with release v1.2.3 and resets the curl log.
#   2. Breaks the mirror or GitHub as the test needs, and runs install.sh into a
#      fresh install directory.
#   3. Checks the exit status, the installed binary and which hosts were used.
#
# Nothing here touches the network or your shell profile.
#
# Usage:
#   scripts/install_test.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SH="${SCRIPT_DIR}/install.sh"

MIRROR_URL="https://mirror.test/.agents/cli"

# Set by main.
ROOT=""
ARCHIVE=""
CHECKSUMS="tollbit_1.2.3_checksums.txt"
FAILURES=0

# Set by setup_fixture, per test.
MIRROR=""
GITHUB=""
INSTALL_DIR=""
LOG=""
OUTPUT=""

fail() {
  echo "    FAIL: $*" >&2
  FAILURES=$((FAILURES + 1))
}

assert_eq() {
  local want="$1"
  local got="$2"
  local what="$3"

  [[ "$want" == "$got" ]] || fail "$what: want '$want', got '$got'"
}

assert_output_contains() {
  local want="$1"

  grep -q -F -e "$want" <<<"$OUTPUT" || fail "output does not contain '$want':"$'\n'"$OUTPUT"
}

# Counts the logged requests to URLs that start with $1.
count_requests() {
  awk -v prefix="$1" 'index($NF, prefix) == 1' "$LOG" | wc -l | tr -d ' '
}

assert_installed() {
  local want="$1"

  assert_eq "$want" "$("${INSTALL_DIR}/tollbit" version 2>/dev/null || true)" "installed version"
}

# Writes a fake curl that logs the URL (last on the line) and serves it from
# FAKE_MIRROR or FAKE_GITHUB, to -o's file or stdout. A URL with no file fails
# like a 404. FAKE_MIRROR_DOWN fails every mirror request like a refused
# connection.
write_fake_curl() {
  local bin="$1"

  mkdir -p "$bin"
  cat >"${bin}/curl" <<'FAKE'
#!/usr/bin/env bash
out=""
url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; shift ;;
    --connect-timeout|--speed-time|--speed-limit) shift ;;
    -*) ;;
    *) url="$1" ;;
  esac
  shift
done
echo "curl $url" >>"$FAKE_LOG"
case "$url" in
  https://mirror.test/.agents/cli/*)
    if [[ -n "${FAKE_MIRROR_DOWN:-}" ]]; then
      echo "curl: (7) Failed to connect to mirror.test port 443" >&2
      exit 7
    fi
    file="${FAKE_MIRROR}/${url#https://mirror.test/.agents/cli/}"
    ;;
  https://api.github.com/repos/*/releases/latest)
    file="${FAKE_GITHUB}/api-latest.json"
    ;;
  https://github.com/*/releases/download/*)
    file="${FAKE_GITHUB}/${url#https://github.com/*/releases/download/}"
    ;;
  *)
    echo "fake curl: unexpected URL $url" >&2
    exit 6
    ;;
esac
if [[ ! -f "$file" ]]; then
  echo "curl: (22) The requested URL returned error: 404" >&2
  exit 22
fi
if [[ -n "$out" ]]; then
  cp "$file" "$out"
else
  cat "$file"
fi
FAKE
  chmod +x "${bin}/curl"
}

# Builds the v1.2.3 archive for this machine, with a tollbit that prints its
# version.
build_archive() {
  local os arch dir

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    *) arch="arm64" ;;
  esac
  ARCHIVE="tollbit_1.2.3_${os}_${arch}.tar.gz"

  dir="${ROOT}/build"
  mkdir -p "$dir"
  printf '#!/bin/sh\necho 1.2.3\n' >"${dir}/tollbit"
  chmod +x "${dir}/tollbit"
  tar -czf "${ROOT}/${ARCHIVE}" -C "$dir" tollbit
}

# Writes release v1.2.3 into $1: the archive and its checksums file.
write_release() {
  local dir="$1/v1.2.3"

  mkdir -p "$dir"
  cp "${ROOT}/${ARCHIVE}" "${dir}/${ARCHIVE}"
  (cd "$dir" && sha256sum "$ARCHIVE" >"$CHECKSUMS")
}

setup_fixture() {
  MIRROR="${ROOT}/mirror"
  GITHUB="${ROOT}/github"
  INSTALL_DIR="${ROOT}/install"
  LOG="${ROOT}/curl.log"
  OUTPUT=""

  rm -rf "$MIRROR" "$GITHUB" "$INSTALL_DIR"
  : >"$LOG"
  write_release "$MIRROR"
  printf 'version=v1.2.3\n' >"${MIRROR}/latest"
  write_release "$GITHUB"
  printf '{"tag_name": "v1.2.3"}\n' >"${GITHUB}/api-latest.json"
}

# Runs install.sh against the fixture with "$@". Sets OUTPUT; returns its exit
# status.
run_install() {
  local rc=0

  OUTPUT="$(
    PATH="${ROOT}/bin:${PATH}" HOME="${ROOT}/home" \
      FAKE_LOG="$LOG" FAKE_MIRROR="$MIRROR" FAKE_GITHUB="$GITHUB" \
      bash "$INSTALL_SH" --base-url "$MIRROR_URL" --install-dir "$INSTALL_DIR" --no-modify-path "$@" 2>&1
  )" || rc=$?
  return "$rc"
}

test_installs_latest_from_the_mirror() {
  run_install || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_eq 3 "$(count_requests "$MIRROR_URL")" "mirror requests"
  assert_eq 0 "$(count_requests https://api.github.com)" "GitHub API requests"
  assert_eq 0 "$(count_requests https://github.com)" "GitHub requests"
}

test_installs_a_named_version_from_the_mirror() {
  rm "${MIRROR}/latest"

  run_install --version v1.2.3 || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_eq 0 "$(count_requests https://github.com)" "GitHub requests"
}

test_latest_ignores_keys_it_does_not_know() {
  printf 'channel=stable\nversion=v1.2.3\nnext=v9.9.9\n' >"${MIRROR}/latest"

  run_install || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_eq 0 "$(count_requests https://github.com)" "GitHub requests"
}

test_unreachable_mirror_falls_back_to_github() {
  FAKE_MIRROR_DOWN=1 run_install || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_output_contains "Could not download from ${MIRROR_URL}; trying GitHub instead."
  assert_eq 1 "$(count_requests https://api.github.com)" "GitHub API requests"
  assert_eq 2 "$(count_requests https://github.com)" "GitHub requests"
}

test_version_missing_from_mirror_falls_back_to_github() {
  rm -r "${MIRROR}/v1.2.3"

  run_install --version v1.2.3 || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_eq 0 "$(count_requests https://api.github.com)" "GitHub API requests"
  assert_eq 2 "$(count_requests https://github.com)" "GitHub requests"
}

test_latest_without_a_version_falls_back_to_github() {
  printf '<html>blocked</html>\n' >"${MIRROR}/latest"

  run_install || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_output_contains "${MIRROR_URL}/latest has no version=v... line"
  assert_eq 1 "$(count_requests https://api.github.com)" "GitHub API requests"
}

test_mirror_checksum_mismatch_fails_without_fallback() {
  echo "tampered" >>"${MIRROR}/v1.2.3/${ARCHIVE}"

  if run_install; then
    fail "install succeeded"
  fi
  assert_output_contains "checksum mismatch for ${ARCHIVE}"
  assert_eq 0 "$(count_requests https://github.com)" "GitHub requests"
  [[ ! -e "${INSTALL_DIR}/tollbit" ]] || fail "installed a binary"
}

test_other_repo_uses_only_github() {
  run_install --repo tollbit/cli-prereleases || fail "install failed: $OUTPUT"

  assert_installed 1.2.3
  assert_eq 0 "$(count_requests "$MIRROR_URL")" "mirror requests"
  assert_eq 2 "$(count_requests https://github.com/tollbit/cli-prereleases/)" "GitHub requests"
}

test_github_failure_after_fallback_fails() {
  rm "${GITHUB}/v1.2.3/${ARCHIVE}"

  if FAKE_MIRROR_DOWN=1 run_install; then
    fail "install succeeded"
  fi
  [[ ! -e "${INSTALL_DIR}/tollbit" ]] || fail "installed a binary"
}

run_tests() {
  local name

  while read -r name; do
    echo "--- ${name}"
    setup_fixture
    "$name"
  done < <(declare -F | awk '$3 ~ /^test_/ { print $3 }')
}

main() {
  ROOT="$(mktemp -d)"
  trap 'rm -rf "$ROOT"' EXIT

  mkdir -p "${ROOT}/home"
  write_fake_curl "${ROOT}/bin"
  build_archive
  run_tests

  if [[ "$FAILURES" -gt 0 ]]; then
    echo "${FAILURES} failure(s)" >&2
    exit 1
  fi
  echo "all tests passed"
}

main "$@"
