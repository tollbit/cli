# Contributing

Development and release notes for the Tollbit CLI (`tollbit/cli`).

## Development

**CI:** `go test ./...` runs on pull requests and on pushes to `main` via [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (Go version from [`go.mod`](go.mod)).

`tollbit guide` is an embedded copy of [`skill/tollbit-cli/SKILL.md`](skill/tollbit-cli/SKILL.md) — edit that file only; do not maintain a separate guide.

The repo includes a small `Makefile`:

| Target | What it runs |
|--------|----------------|
| `make test` | `go test ./...` |
| `make build` | `go build -o tollbit ./cmd/tollbit` (binary at `./tollbit` in the repo root) |
| `make alias` | Prints a one-line `alias` so you can point `tollbit` at that binary |
| `make dev-install` | Builds and installs the repo binary into your `tollbit` command path, moving any existing installed binary to `tollbit-original` |
| `make dev-uninstall` | Restores `tollbit-original` back to `tollbit` and removes the dev-installed binary |
| `make bump VERSION=…` | Bumps `internal/version`, `skill/tollbit-cli/SKILL.md`, and install examples in [README.md](README.md) |
| `make tag` | Creates `v$(Version)` from [`internal/version/version.go`](internal/version/version.go); use `ALLOW_DIRTY=1` to skip the clean-tree check |

```bash
make test
make build
./tollbit --help
```

Swap your built binary into your installed `tollbit` command, then restore it later:

```bash
make dev-install
tollbit --version

make dev-uninstall
tollbit --version
```

`make` runs in a subprocess, so it cannot define an alias in your current shell by itself. After `make build`, run:

```bash
eval "$(make alias)"
```

That defines `tollbit` for the rest of the session (for example: `tollbit --help`).

### Local configuration

For local development, point `TOLLBIT_ENV_FILE` at a dotenv file to load it at startup — for example `export TOLLBIT_ENV_FILE=.env` in your shell or direnv (a relative path resolves against the current directory). A `.env` is **not** auto-discovered from the working directory, so a stray `.env` in an unrelated repo can never take effect. Only `TOLLBIT_`-prefixed keys are honored, and existing shell variables are not overwritten. Useful vars include `TOLLBIT_AUTH_BASE_URL`, `TOLLBIT_GATEWAY_BASE_URL`, `TOLLBIT_AGENT_DEFAULT_NAME`, `TOLLBIT_CREDENTIALS_STORAGE_DIR`, and `TOLLBIT_LOG_LEVEL`.

## Release

Publishing is triggered by pushing a **version tag** whose name starts with **`v`**. The semver must match [`internal/version/version.go`](internal/version/version.go); [`skill/tollbit-cli/SKILL.md`](skill/tollbit-cli/SKILL.md) must stay aligned (`go test ./...` enforces this).

Do this in two steps: land the version bump on `main` via PR, then tag and push from `main`. Do **not** create or push the release tag from a feature branch — if the PR is squashed or rebased, that tag may not point at the commit that ends up on `main`.

### 1. Version bump PR

On a branch from an up-to-date `main`, bump the repo together (CLI const, skill frontmatter/body, and pinned **`v…`** install examples in [README.md](README.md)):

```bash
git checkout -b release/v0.2.0
make bump VERSION=0.2.0
make test
```

Commit the bump, open a PR, and merge it to `main` after CI is green.

### 2. Tag from `main`

After the bump is on `main`, check out `main`, pull, and create the tag from [`internal/version/version.go`](internal/version/version.go) (must match — refuses if the tag already exists or the tree is dirty; override with `ALLOW_DIRTY=1` if you must):

```bash
git checkout main
git pull origin main
make tag
git push origin vX.Y.Z
```

Use the printed tag name in the push (same as `v` + `Version` in code). Only the **push of the tag** starts the release workflow.

### Release workflow

The release pipeline lives in [`.github/workflows/release.yml`](.github/workflows/release.yml). On each tag push it:

1. Runs [GoReleaser](.goreleaser.yaml) to build cross-platform binaries and publish a **GitHub Release on this repo** (`tollbit/cli`).
2. Publishes the npm wrapper [`@tollbit/tollbit-cli`](https://www.npmjs.com/package/@tollbit/tollbit-cli) (OIDC trusted publishing; no separate npm token when configured).

Install scripts live at `scripts/install.{sh,ps1}` on `main` in this repo; users and the npm postinstall step download binaries from this repo's GitHub Releases.

npm **trusted publishing** must be linked to `tollbit/cli` and this workflow (`release.yml`). The old dual-repo model (`tollbit-cli-releases` + `RELEASES_GITHUB_TOKEN` + `release-public/` sync) has been removed. New releases publish only to `tollbit/cli`.

To move or reuse a tag locally before pushing:

```bash
git tag -d v0.1.0
git tag v0.1.0
```

If you already pushed the wrong tag, fix it on the remote only if your policy allows force-deleting release tags.

