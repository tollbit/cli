#!/usr/bin/env bash
set -euo pipefail

# Prints the body of one version's section from a Keep a Changelog file:
#   1. Find the "## [X.Y.Z]" heading (a date after it is optional).
#   2. Print everything up to the next "## [" heading or the link reference
#      definitions at the bottom of the file.
#   3. Trim leading and trailing blank lines.
#
# Exits non-zero if the section is missing or empty, so release tooling can use
# it as a gate.
#
# Usage:
#   scripts/changelog-section.sh 0.3.6 [CHANGELOG.md]
#   scripts/changelog-section.sh v0.3.6 - < CHANGELOG.md

usage() {
  echo "usage: $0 <version> [changelog-file|-]" >&2
  exit 2
}

# Prints the section body for a version, read from stdin.
extract_section() {
  local version="$1"

  awk -v want="$version" '
    /^## \[/ {
      if (found) exit
      heading = $0
      sub(/^## \[/, "", heading)
      sub(/\].*$/, "", heading)
      if (heading == want) { found = 1; next }
    }
    /^\[[^]]+\]: / { if (found) exit }
    found {
      lines[++n] = $0
      if ($0 ~ /[^[:space:]]/) { if (!first) first = n; last = n }
    }
    END { for (i = first; first && i <= last; i++) print lines[i] }
  '
}

main() {
  [[ $# -ge 1 && $# -le 2 ]] || usage

  local version="${1#v}"
  local file="${2:-CHANGELOG.md}"
  local body
  body="$(cat -- "$file" | extract_section "$version")"

  if [[ -z "$body" ]]; then
    echo "error: no non-empty section for version ${version} in ${file}" >&2
    exit 1
  fi

  printf '%s\n' "$body"
}

main "$@"
