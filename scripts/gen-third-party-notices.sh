#!/usr/bin/env bash
# Regenerates THIRD-PARTY-NOTICES.md from the licenses of the Go dependencies
# linked into the tollbit binary. Run after any dependency change.
set -euo pipefail
cd "$(dirname "$0")/.."

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

go run github.com/google/go-licenses@v1.6.0 save ./cmd/tollbit --save_path="${TMP}/licenses" --force

OUT=THIRD-PARTY-NOTICES.md
{
  echo "# Third-Party Notices"
  echo
  echo "The tollbit binary includes the following third-party software, used under"
  echo "their respective licenses."
  find "${TMP}/licenses" -type f | sort | while read -r f; do
    rel="${f#"${TMP}/licenses/"}"
    mod="$(dirname "${rel}")"
    base="$(basename "${rel}")"
    # Exclude our own module — this file is for third-party notices only.
    case "${mod}" in
      github.com/tollbit/cli | github.com/tollbit/cli/*) continue ;;
    esac
    echo
    echo "---"
    echo
    echo "## ${mod} — ${base}"
    echo
    echo '```'
    cat "${f}"
    echo '```'
  done
} > "${OUT}"

echo "wrote ${OUT}"
