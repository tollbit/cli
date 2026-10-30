#!/usr/bin/env python3
"""Bump internal/version.Version and the pinned install examples in README.

The bundled skills take their version from the CLI when `tollbit guide` renders
them ({{ .Version }}), so they need no bump.
"""

from __future__ import annotations

import pathlib
import re
import sys


def main() -> None:
    if len(sys.argv) != 2:
        print("usage: bump-version.py X.Y.Z   or   vX.Y.Z", file=sys.stderr)
        sys.exit(2)

    new = sys.argv[1].removeprefix("v").strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", new):
        print("error: expected semver X.Y.Z", file=sys.stderr)
        sys.exit(2)

    root = pathlib.Path(__file__).resolve().parent.parent
    version_go = root / "internal" / "version" / "version.go"
    readme = root / "README.md"

    text = version_go.read_text()
    m = re.search(r'^const Version = "([^"]*)"', text, re.M)
    if not m:
        print(f"error: could not parse const Version in {version_go}", file=sys.stderr)
        sys.exit(1)
    old = m.group(1)

    if old == new:
        print(f"already at {new}")
        return

    version_go.write_text(
        text.replace(f'const Version = "{old}"', f'const Version = "{new}"', 1)
    )


    rt = readme.read_text()
    rt_new = rt.replace(f"v{old}", f"v{new}")
    readme.write_text(rt_new)

    print(f"bumped {old} -> {new}")
    print("run: make test")


if __name__ == "__main__":
    main()
