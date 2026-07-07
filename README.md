# Tollbit CLI

Tollbit's cli is a CLI for Grounding Agents on ready to license content using the TollBit network

Primary workflow: `auth login` → confirm with `auth status` → search content with `search`.

## Install

### macOS and Linux (installer)

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash
```

Optional installer flags:

```bash
# Install a pinned version
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash -s -- --version v0.0.1

# Install to a custom path
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash -s -- --install-dir "$HOME/bin" --force
```

PATH behavior:

- Installer defaults to `~/.local/bin`.
- If needed, installer adds that directory to your shell profile (`.zshrc`, `.bashrc`, or `.profile`) once.
- Use `--no-modify-path` to avoid PATH changes in CI or locked environments.

### Windows (PowerShell installer)

```powershell
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
```

Optional installer flags:

```powershell
# Install a pinned version
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
Install-Tollbit -Version v0.0.1 -Force

# Install without PATH mutation
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
Install-Tollbit -NoModifyPath -PrintPathInstructions
```

### npm

```bash
npm install -g @tollbit/tollbit-cli
```

### Manual fallback

1. Download the platform archive from the [tollbit-cli-releases](https://github.com/tollbit/tollbit-cli-releases) GitHub release for your tag.
2. Verify SHA-256 using `tollbit_<version>_checksums.txt`.
3. Extract `tollbit` (`tollbit.exe` on Windows) into a directory on your PATH.

### Build from source

```bash
go build ./cmd/tollbit
```

**Public distribution repo:** On each tag, CI syncs **`release-public/README.md`** → **`README.md`**, plus **`scripts/install.{sh,ps1}`**, to [`tollbit/tollbit-cli-releases`](https://github.com/tollbit/tollbit-cli-releases) **`main`**, then **mirrors the same tag** (e.g. **`v0.1.0`**) onto that **`main`** commit **before** GoReleaser runs. GitHub requires both a non-empty repo and the release tag to exist on the publication repo. Edit the canonical copies here.

## Configure

Agent credentials are stored under `TOLLBIT_CREDENTIALS_STORAGE_DIR` (default platform path): `agent-identity.json` and `agent-token.jwt`.

For local development, a `.env` file in the current working directory is loaded at startup (existing shell variables are not overwritten). Override the path with `TOLLBIT_ENV_FILE`. Useful vars include `TOLLBIT_AUTH_BASE_URL`, `TOLLBIT_GATEWAY_BASE_URL`, `TOLLBIT_AGENT_DEFAULT_NAME`, `TOLLBIT_AGENT_TOKEN`, `TOLLBIT_CREDENTIALS_STORAGE_DIR`, and `TOLLBIT_LOG_LEVEL`.

## Commands

Agent? Run `./tollbit guide`.

### Auth

Manage your agent profile and authorization token:

```bash
./tollbit auth login --name my-agent --user-agent MyAgent-User
./tollbit auth status
./tollbit auth status --json
./tollbit auth status --check
./tollbit auth set --name my-agent --user-agent MyAgent-User
./tollbit auth logout
./tollbit auth logout --all
```

`TOLLBIT_AGENT_DEFAULT_NAME` and `TOLLBIT_AGENT_DEFAULT_USER_AGENT` set fallback profile defaults. Saved profile overrides those defaults. `search` and `content` accept `--user-agent` as a per-request override.

`auth status --check` exits `0` when the token is valid, `1` when invalid/expired, and `2` when missing (no stdout). For CI, inject a token with `TOLLBIT_AGENT_TOKEN` instead of minting interactively.

Set `TOLLBIT_AUTH_BROWSER_CONSENT_AUTO_OPEN_BROWSER=false` in headless environments. Confirm readiness with `auth status` before search calls.

### Search

Search publisher content via the gateway API:

```bash
./tollbit search "climate policy"
./tollbit search "climate policy" --size 10 --json
./tollbit search "query" --properties example.com,cnn.com
./tollbit search --help
```

Environment:

- `TOLLBIT_GATEWAY_BASE_URL` — gateway base URL (default `https://gateway.tollbit.com`)

### Content

Price and fetch licensed publisher content:

```bash
./tollbit content pricing https://example.com/article-1,https://example.com/article-2
./tollbit content pricing https://example.com/article --json
./tollbit content fetch https://example.com/article
./tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
./tollbit content fetch https://example.com/article --confirm --json --rate-index 1
```

Known license types show consumer-facing labels (for example `Summarization (ON_DEMAND_LICENSE)`).

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost). Use `--toDisk=<path>` to save fetched content locally. Set a registered user agent with `auth set --user-agent` or `--user-agent` on the fetch command.

### Guide

Print the built-in agent orientation guide (embedded at build time):

```bash
./tollbit guide
```

Install it into a harness skill directory:

```bash
# Parent skills directory (writes <dir>/<skill-name>/SKILL.md; name from embedded frontmatter, e.g. tollbit-cli)
./tollbit guide --install ~/.claude/skills
```

### Version

Print the CLI release (compare to the `version` field in the embedded skill / `skill/tollbit-cli/SKILL.md` when deciding whether to refresh an installed skill):

```bash
./tollbit version
./tollbit --version
```

`./tollbit help` also prints this version.

## Development

**CI:** **`go test ./...`** runs on **pull requests** (including each push to the PR branch) and on **pushes to `main`** via [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (Go version from [`go.mod`](go.mod)). Feature branches only run through PRs so the workflow does not double-fire; use a draft PR or **Actions → Run workflow** if you need CI before opening a PR.

The repo includes a small `Makefile`:

| Target | What it runs |
|--------|----------------|
| `make test` | `go test ./...` |
| `make build` | `go build -o tollbit ./cmd/tollbit` (binary at `./tollbit` in the repo root) |
| `make alias` | Prints a one-line `alias` so you can point `tollbit` at that binary |
| `make dev-install` | Builds and installs the repo binary into your `tollbit` command path, moving any existing installed binary to `tollbit-original` |
| `make dev-uninstall` | Restores `tollbit-original` back to `tollbit` and removes the dev-installed binary |
| `make bump VERSION=…` | Bumps `internal/version`, `skill/tollbit-cli/SKILL.md`, and install examples in this README (see **Release**) |
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

## Release

Publishing is triggered by pushing a **version tag** whose name starts with **`v`**. The semver must match [`internal/version/version.go`](internal/version/version.go); [`skill/tollbit-cli/SKILL.md`](skill/tollbit-cli/SKILL.md) must stay aligned (`go test ./...` enforces this).

Bump the repo together (CLI const, skill frontmatter/body, and pinned **`v…`** install examples in this README):

```bash
make bump VERSION=0.2.0
make test
```

Then commit the version bump (and any other release commits). Create the tag from [`internal/version/version.go`](internal/version/version.go) (must match — refuses if the tag already exists or the tree is dirty; override with `ALLOW_DIRTY=1` if you must):

```bash
make tag
git push origin vX.Y.Z
```

Use the printed tag name in the push (same as `v` + `Version` in code). Only the **`push` of the tag** starts the release workflow ([`.github/workflows/release.yml`](.github/workflows/release.yml)): CI syncs [`release-public/README.md`](release-public/README.md) and the install scripts to the public repo **first**, then GoReleaser uploads binaries to [`tollbit/tollbit-cli-releases`](https://github.com/tollbit/tollbit-cli-releases), then npm publish runs if configured.

To move or reuse a tag locally before pushing:

```bash
git tag -d v0.1.0
git tag v0.1.0
```

If you already pushed the wrong tag, fix it on the remote only if your policy allows force-deleting release tags.

## Updating

Installer channel updates:

```bash
# Latest
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash

# Pinned
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash -s -- --version v0.0.1 --force
```

```powershell
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
Install-Tollbit -Version latest -Force
```

Other channels:

- npm: `npm update -g @tollbit/tollbit-cli`

## Assumptions

- Gateway API base URL defaults to `https://gateway.tollbit.com` (agent search).
- Agent identity tokens are minted via the auth service (`TOLLBIT_AUTH_BASE_URL`).
