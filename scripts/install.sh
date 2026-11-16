#!/usr/bin/env bash
set -euo pipefail

# Installs the tollbit CLI from tollbit.com, or from GitHub Releases when
# tollbit.com can't be reached:
#   1. Download from tollbit.com: the version `latest` names (unless --version
#      names one), then the archive for this OS and architecture and the
#      checksums.
#   2. If any of that fails, do the same from GitHub Releases. With --repo set
#      to another repository, only GitHub is used.
#   3. Verify the archive's SHA-256 and extract the binary. A mismatch fails the
#      install: it never falls back, so a bad copy can't go unnoticed.
#   4. Install it, unless the same version is already there (or --force).
#   5. Record the install method and add the install directory to PATH.
#
# Everything runs from main, called on the last line, so a download cut short
# by `curl | bash` runs nothing.
#
# Usage:
#   install.sh [options]   # see usage below
#
# Overridable env vars:
#   REPO  (default tollbit/cli) GitHub repository; --repo wins

DEFAULT_REPO="tollbit/cli"
REPO="${REPO:-${DEFAULT_REPO}}"
BINARY_NAME="tollbit"

# A blocked or dropped connection to tollbit.com should fall back to GitHub
# quickly, not hang.
MIRROR_CURL_OPTS=(--connect-timeout 10 --speed-time 30 --speed-limit 1)
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"

# Set by parse_args.
VERSION="latest"
BASE_URL="https://tollbit.com/.agents/cli"
INSTALL_DIR="${DEFAULT_INSTALL_DIR}"
FORCE=0
NO_MODIFY_PATH=0
PRINT_PATH_INSTRUCTIONS=0

# Set by main.
OS=""
ARCH=""
TMP_DIR=""

usage() {
  cat <<'EOF'
Install tollbit CLI from tollbit.com, or GitHub Releases if tollbit.com
can't be reached.

Usage:
  install.sh [options]

Options:
  --version <vX.Y.Z|latest>     Version to install (default: latest)
  --install-dir <path>          Install directory (default: ~/.local/bin)
  --force                       Overwrite an existing binary
  --no-modify-path              Do not update shell profile PATH
  --print-path-instructions     Print PATH instructions even if PATH is updated
  --base-url <url>              Where to download from first
                                (default: https://tollbit.com/.agents/cli)
  --repo <owner/repo>           GitHub repository (default: tollbit/cli);
                                any other skips tollbit.com
  -h, --help                    Show this help
EOF
}

fail() {
  echo "error: $*" >&2
  exit 1
}

log() {
  echo "$*"
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --version)
        VERSION="${2:-}"
        shift 2
        ;;
      --install-dir)
        INSTALL_DIR="${2:-}"
        shift 2
        ;;
      --base-url)
        BASE_URL="${2:-}"
        BASE_URL="${BASE_URL%/}"
        shift 2
        ;;
      --force)
        FORCE=1
        shift
        ;;
      --no-modify-path)
        NO_MODIFY_PATH=1
        shift
        ;;
      --print-path-instructions)
        PRINT_PATH_INSTRUCTIONS=1
        shift
        ;;
      --repo)
        REPO="${2:-}"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
  done

  [[ -n "${VERSION}" ]] || fail "--version requires a value"
  [[ -n "${INSTALL_DIR}" ]] || fail "--install-dir requires a value"
}

check_tools() {
  have_cmd curl || fail "curl is required"
  have_cmd tar || fail "tar is required"
  if ! have_cmd shasum && ! have_cmd sha256sum; then
    fail "shasum or sha256sum is required"
  fi
}

detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "${OS}" in
    darwin|linux) ;;
    *) fail "unsupported OS: ${OS}; use the PowerShell installer for Windows" ;;
  esac

  case "${ARCH}" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) fail "unsupported architecture: ${ARCH}" ;;
  esac
}

# Fails unless $1 is vX.Y.Z or latest.
check_version() {
  case "$1" in
    v*|latest) ;;
    *) fail "version must begin with 'v' (example: v0.0.1) or be 'latest'" ;;
  esac
}

archive_name() {
  echo "${BINARY_NAME}_${1#v}_${OS}_${ARCH}.tar.gz"
}

checksums_name() {
  echo "${BINARY_NAME}_${1#v}_checksums.txt"
}

uses_mirror() {
  [[ "${REPO}" == "${DEFAULT_REPO}" && -n "${BASE_URL}" ]]
}

mirror_fetch() {
  curl "${MIRROR_CURL_OPTS[@]}" -fsSL "$1" -o "$2"
}

# Prints the version BASE_URL/latest names (its version=vX.Y.Z line), or
# fails. Other lines are ignored.
mirror_latest_version() {
  local version

  mirror_fetch "${BASE_URL}/latest" "${TMP_DIR}/latest" || return 1
  version="$(sed -n 's/^version=//p' "${TMP_DIR}/latest" | tr -d '\r' | head -n 1)"
  if [[ "${version}" != v* ]]; then
    echo "${BASE_URL}/latest has no version=v... line" >&2
    return 1
  fi
  echo "${version}"
}

# Downloads the archive and checksums from BASE_URL and sets VERSION. Fails
# without changing VERSION if anything can't be downloaded.
download_from_mirror() {
  local version="${VERSION}"
  local base_url

  if [[ "${version}" == "latest" ]]; then
    version="$(mirror_latest_version)" || return 1
  fi
  base_url="${BASE_URL}/${version}"

  log "Downloading $(archive_name "${version}") from ${BASE_URL}..."
  mirror_fetch "${base_url}/$(archive_name "${version}")" "${TMP_DIR}/$(archive_name "${version}")" || return 1
  mirror_fetch "${base_url}/$(checksums_name "${version}")" "${TMP_DIR}/$(checksums_name "${version}")" || return 1
  VERSION="${version}"
}

github_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1
}

# Downloads the archive and checksums from GitHub Releases and sets VERSION.
download_from_github() {
  local base_url

  if [[ "${VERSION}" == "latest" ]]; then
    VERSION="$(github_latest_version)"
    [[ -n "${VERSION}" ]] || fail "failed to resolve latest version"
    check_version "${VERSION}"
  fi
  base_url="https://github.com/${REPO}/releases/download/${VERSION}"

  log "Downloading $(archive_name "${VERSION}") from GitHub..."
  curl -fsSL "${base_url}/$(archive_name "${VERSION}")" -o "${TMP_DIR}/$(archive_name "${VERSION}")"
  curl -fsSL "${base_url}/$(checksums_name "${VERSION}")" -o "${TMP_DIR}/$(checksums_name "${VERSION}")"
}

download_release() {
  if uses_mirror; then
    if download_from_mirror; then
      return
    fi
    log "Could not download from ${BASE_URL}; trying GitHub instead."
  fi
  download_from_github
}

sha256_of() {
  if have_cmd shasum; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

verify_archive() {
  local archive expected

  archive="$(archive_name "${VERSION}")"
  expected="$(awk -v file="${archive}" '$2 == file { print $1 }' "${TMP_DIR}/$(checksums_name "${VERSION}")" | head -n 1)"
  [[ -n "${expected}" ]] || fail "checksum entry not found for ${archive}"
  [[ "${expected}" == "$(sha256_of "${TMP_DIR}/${archive}")" ]] || fail "checksum mismatch for ${archive}"
}

# Prints the path of the extracted binary.
extract_binary() {
  mkdir -p "${TMP_DIR}/extract"
  tar -xzf "${TMP_DIR}/$(archive_name "${VERSION}")" -C "${TMP_DIR}/extract"
  [[ -f "${TMP_DIR}/extract/${BINARY_NAME}" ]] || fail "archive did not contain ${BINARY_NAME}"
  echo "${TMP_DIR}/extract/${BINARY_NAME}"
}

# True when the binary at $1 already reports VERSION. Fails when another
# version is there and --force is not set.
is_up_to_date() {
  local dest="$1"
  local current

  [[ -f "${dest}" ]] && [[ "${FORCE}" -ne 1 ]] || return 1

  current="$("${dest}" version 2>/dev/null || true)"
  if [[ "${current}" == "${VERSION#v}" ]]; then
    return 0
  fi
  fail "${dest} already exists; re-run with --force to overwrite"
}

install_binary() {
  local source="$1"
  local dest="$2"
  local temp_dest="${dest}.tmp.$$"

  install -m 0755 "${source}" "${temp_dest}"
  mv -f "${temp_dest}" "${dest}"

  # Record the install method so the CLI can show the right update command.
  printf 'installer\n' > "${INSTALL_DIR}/.tollbit-install-method" || true
}

path_contains_install_dir() {
  case ":${PATH}:" in
    *:"${INSTALL_DIR}":*) return 0 ;;
  esac
  return 1
}

detect_profile_file() {
  if [[ -n "${SHELL:-}" ]] && [[ "${SHELL}" == *zsh* ]]; then
    echo "${HOME}/.zshrc"
    return
  fi
  if [[ -n "${SHELL:-}" ]] && [[ "${SHELL}" == *bash* ]]; then
    echo "${HOME}/.bashrc"
    return
  fi
  if [[ -f "${HOME}/.zshrc" ]]; then
    echo "${HOME}/.zshrc"
    return
  fi
  if [[ -f "${HOME}/.bashrc" ]]; then
    echo "${HOME}/.bashrc"
    return
  fi
  echo "${HOME}/.profile"
}

# Adds INSTALL_DIR to PATH in the shell profile. True when it added the line.
add_to_profile() {
  local profile="$1"
  local export_line="export PATH=\"${INSTALL_DIR}:\$PATH\" # tollbit-installer"

  touch "${profile}"
  if grep -Fq "${export_line}" "${profile}"; then
    return 1
  fi
  printf '\n%s\n' "${export_line}" >> "${profile}"
}

print_summary() {
  local dest="$1"
  local path_present=0
  local path_updated=0
  local profile

  if path_contains_install_dir; then
    path_present=1
  fi
  profile="$(detect_profile_file)"
  if [[ "${NO_MODIFY_PATH}" -ne 1 ]] && [[ "${path_present}" -ne 1 ]] && add_to_profile "${profile}"; then
    path_updated=1
  fi

  log "Installed ${BINARY_NAME} ${VERSION#v} to ${dest}"

  if [[ "${path_present}" -eq 1 ]]; then
    log "PATH already contains ${INSTALL_DIR}"
  elif [[ "${path_updated}" -eq 1 ]]; then
    log "Added ${INSTALL_DIR} to PATH in ${profile}"
    log "Reload your shell: source \"${profile}\""
  fi

  if [[ "${PRINT_PATH_INSTRUCTIONS}" -eq 1 ]] || [[ "${NO_MODIFY_PATH}" -eq 1 ]] || { [[ "${path_present}" -ne 1 ]] && [[ "${path_updated}" -ne 1 ]]; }; then
    log "If '${BINARY_NAME}' is not found, add this line to your shell profile:"
    log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi

  log "Verify install:"
  log "  ${BINARY_NAME} version"
  log "Fallback full path:"
  log "  ${dest} version"
}

cleanup() {
  rm -rf "${TMP_DIR}"
}

main() {
  local binary dest

  parse_args "$@"
  check_tools
  detect_platform
  check_version "${VERSION}"

  TMP_DIR="$(mktemp -d)"
  trap cleanup EXIT

  download_release
  verify_archive
  binary="$(extract_binary)"

  mkdir -p "${INSTALL_DIR}"
  dest="${INSTALL_DIR}/${BINARY_NAME}"
  if is_up_to_date "${dest}"; then
    log "${BINARY_NAME} ${VERSION#v} is already up to date at ${dest}"
    exit 0
  fi
  install_binary "${binary}" "${dest}"
  print_summary "${dest}"
}

main "$@"
