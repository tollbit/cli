#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-tollbit/tollbit-cli-releases}"
BINARY_NAME="tollbit"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"
VERSION="latest"
INSTALL_DIR="${DEFAULT_INSTALL_DIR}"
FORCE=0
NO_MODIFY_PATH=0
PRINT_PATH_INSTRUCTIONS=0

usage() {
  cat <<'EOF'
Install tollbit CLI from GitHub Releases.

Usage:
  install.sh [options]

Options:
  --version <vX.Y.Z|latest>     Version to install (default: latest)
  --install-dir <path>          Install directory (default: ~/.local/bin)
  --force                       Overwrite an existing binary
  --no-modify-path              Do not update shell profile PATH
  --print-path-instructions     Print PATH instructions even if PATH is updated
  --repo <owner/repo>           GitHub repository (default: tollbit/tollbit-cli-releases)
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

if ! have_cmd curl; then
  fail "curl is required"
fi
if ! have_cmd tar; then
  fail "tar is required"
fi
if ! have_cmd shasum && ! have_cmd sha256sum; then
  fail "shasum or sha256sum is required"
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${OS}" in
  darwin|linux) ;;
  *)
    fail "unsupported OS: ${OS}; use the PowerShell installer for Windows"
    ;;
esac

case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) fail "unsupported architecture: ${ARCH}" ;;
esac

resolve_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1
}

if [[ "${VERSION}" == "latest" ]]; then
  VERSION="$(resolve_latest_version)"
  [[ -n "${VERSION}" ]] || fail "failed to resolve latest version"
fi

case "${VERSION}" in
  v*) ;;
  *) fail "version must begin with 'v' (example: v0.0.1) or be 'latest'" ;;
esac

ASSET_BASE="${BINARY_NAME}_${VERSION#v}_${OS}_${ARCH}"
ARCHIVE_NAME="${ASSET_BASE}.tar.gz"
CHECKSUMS_NAME="${BINARY_NAME}_${VERSION#v}_checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE_NAME}"
CHECKSUMS_URL="${BASE_URL}/${CHECKSUMS_NAME}"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

ARCHIVE_PATH="${TMP_DIR}/${ARCHIVE_NAME}"
CHECKSUMS_PATH="${TMP_DIR}/${CHECKSUMS_NAME}"

log "Downloading ${ARCHIVE_NAME}..."
curl -fsSL "${ARCHIVE_URL}" -o "${ARCHIVE_PATH}"
curl -fsSL "${CHECKSUMS_URL}" -o "${CHECKSUMS_PATH}"

EXPECTED_SUM="$(awk -v file="${ARCHIVE_NAME}" '$2 == file { print $1 }' "${CHECKSUMS_PATH}" | head -n 1)"
[[ -n "${EXPECTED_SUM}" ]] || fail "checksum entry not found for ${ARCHIVE_NAME}"

if have_cmd shasum; then
  ACTUAL_SUM="$(shasum -a 256 "${ARCHIVE_PATH}" | awk '{print $1}')"
else
  ACTUAL_SUM="$(sha256sum "${ARCHIVE_PATH}" | awk '{print $1}')"
fi

[[ "${EXPECTED_SUM}" == "${ACTUAL_SUM}" ]] || fail "checksum mismatch for ${ARCHIVE_NAME}"

mkdir -p "${TMP_DIR}/extract"
tar -xzf "${ARCHIVE_PATH}" -C "${TMP_DIR}/extract"
EXTRACTED_BIN="${TMP_DIR}/extract/${BINARY_NAME}"
[[ -f "${EXTRACTED_BIN}" ]] || fail "archive did not contain ${BINARY_NAME}"

mkdir -p "${INSTALL_DIR}"
DEST_PATH="${INSTALL_DIR}/${BINARY_NAME}"

if [[ -f "${DEST_PATH}" ]] && [[ "${FORCE}" -ne 1 ]]; then
  CURRENT_VERSION="$("${DEST_PATH}" version 2>/dev/null || true)"
  if [[ "${CURRENT_VERSION}" == "${VERSION#v}" ]]; then
    log "${BINARY_NAME} ${CURRENT_VERSION} is already up to date at ${DEST_PATH}"
    exit 0
  fi
  fail "${DEST_PATH} already exists; re-run with --force to overwrite"
fi

TEMP_DEST="${DEST_PATH}.tmp.$$"
install -m 0755 "${EXTRACTED_BIN}" "${TEMP_DEST}"
mv -f "${TEMP_DEST}" "${DEST_PATH}"

# Record the install method so the CLI can show the right update command.
printf 'installer\n' > "${INSTALL_DIR}/.tollbit-install-method" || true

PATH_ENTRY_PRESENT=0
case ":${PATH}:" in
  *:"${INSTALL_DIR}":*) PATH_ENTRY_PRESENT=1 ;;
esac

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

PROFILE_FILE="$(detect_profile_file)"
EXPORT_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\" # tollbit-installer"
PATH_UPDATED=0

if [[ "${NO_MODIFY_PATH}" -ne 1 ]] && [[ "${PATH_ENTRY_PRESENT}" -ne 1 ]]; then
  touch "${PROFILE_FILE}"
  if ! grep -Fq "${EXPORT_LINE}" "${PROFILE_FILE}"; then
    printf '\n%s\n' "${EXPORT_LINE}" >> "${PROFILE_FILE}"
    PATH_UPDATED=1
  fi
fi

log "Installed ${BINARY_NAME} ${VERSION#v} to ${DEST_PATH}"

if [[ "${PATH_ENTRY_PRESENT}" -eq 1 ]]; then
  log "PATH already contains ${INSTALL_DIR}"
elif [[ "${PATH_UPDATED}" -eq 1 ]]; then
  log "Added ${INSTALL_DIR} to PATH in ${PROFILE_FILE}"
  log "Reload your shell: source \"${PROFILE_FILE}\""
fi

if [[ "${PRINT_PATH_INSTRUCTIONS}" -eq 1 ]] || [[ "${NO_MODIFY_PATH}" -eq 1 ]] || { [[ "${PATH_ENTRY_PRESENT}" -ne 1 ]] && [[ "${PATH_UPDATED}" -ne 1 ]]; }; then
  log "If '${BINARY_NAME}' is not found, add this line to your shell profile:"
  log "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

log "Verify install:"
log "  ${BINARY_NAME} version"
log "Fallback full path:"
log "  ${DEST_PATH} version"
