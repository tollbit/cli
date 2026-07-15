# Tollbit CLI

Command-line client for [Tollbit](https://tollbit.com) agent identity, authorization, and publisher content search.

Binaries and installers are published on **[GitHub Releases](https://github.com/tollbit/cli/releases)**.

Primary workflow: `auth login` → confirm with `auth status` → search content with `search`.

## Install

#### Agent install prompt

**Agent?:** Install Tollbit from [github.com/tollbit/cli](https://github.com/tollbit/cli) using the install script or package that your environment needs (below), then run `tollbit guide --install <SKILLS_DIR>` to register the bundled skill.

### macOS and Linux

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash
```

Pin a version or choose an install directory:

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.2.0
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --install-dir "$HOME/bin" --force
```

By default the binary goes under `~/.local/bin`; the installer can add that directory to your shell `PATH`. Use `--no-modify-path` to skip PATH changes in CI or locked environments.

### Windows (PowerShell)

```powershell
irm "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.ps1" | iex
```

Pin a version or skip `PATH` changes (useful in CI):

```powershell
irm "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.ps1" | iex
Install-Tollbit -Version v0.2.0 -Force
Install-Tollbit -NoModifyPath -PrintPathInstructions
```

### npm

```bash
npm install -g @tollbit/tollbit-cli
```

Then run `tollbit` from any terminal (the package downloads the native binary for your OS).

### Manual install

From **[GitHub Releases](https://github.com/tollbit/cli/releases)**, download the archive for your OS and CPU, verify SHA-256 against `tollbit_<version>_checksums.txt`, and put `tollbit` (or `tollbit.exe`) on your `PATH`.

### Build from source

```bash
go build ./cmd/tollbit
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for local development and release instructions.

## Configure

Agent credentials are stored under `TOLLBIT_CREDENTIALS_STORAGE_DIR` (default platform path): `agent-identity.json` and `agent-token.jwt`.

For local development, a `.env` file in the current working directory is loaded at startup (existing shell variables are not overwritten). Override the path with `TOLLBIT_ENV_FILE`. Useful vars include `TOLLBIT_AUTH_BASE_URL`, `TOLLBIT_GATEWAY_BASE_URL`, `TOLLBIT_AGENT_DEFAULT_NAME`, `TOLLBIT_CREDENTIALS_STORAGE_DIR`, and `TOLLBIT_LOG_LEVEL`.

## For AI coding agents

- Install `tollbit` so it is on **`PATH`** in every environment where you run shell commands (CI images, local sandboxes, agent runners).
- Run **`tollbit guide`** for orientation, then **`tollbit guide --install <SKILLS_DIR>`** to persist the bundled skill.
- Configure agent profile: **`tollbit auth set --name <name>`** (or `TOLLBIT_AGENT_DEFAULT_NAME`), then confirm with **`tollbit auth status`** before search calls.
- Use **`tollbit auth login`** when user/org authorization is required.
- Prefer **`--json`** on **`search`** and **`auth status`** when you need machine-readable output.

## What the CLI can do

| Command | Purpose |
|--------|---------|
| **`auth login/logout/status/set`** | Agent profile and OAuth authorization token. |
| **`search "query"`** | Search publisher content via the gateway API. |
| **`content pricing/fetch`** | Price and fetch licensed publisher content. |
| **`guide`** | Print the agent guide; optionally install bundled skill markdown. |
| **`version`** | Print the CLI version string. |

Typical flow: **`auth login`** → **`auth status`** → **`search "query"`**. Use **`--help`** on each command for flags.

## Commands

### Auth

```bash
tollbit auth login --name my-agent --user-agent MyAgent-User
tollbit auth status
tollbit auth status --json
tollbit auth status --check
tollbit auth set --name my-agent --user-agent MyAgent-User
tollbit auth logout
tollbit auth logout --all
```

`TOLLBIT_AGENT_DEFAULT_NAME` and `TOLLBIT_AGENT_DEFAULT_USER_AGENT` set fallback profile defaults. Saved profile overrides those defaults. `search` and `content` accept `--user-agent` as a per-request override.

`auth status --check` exits `0` when the token is valid, `1` when invalid/expired, and `2` when missing (no stdout).

Set `TOLLBIT_AUTH_BROWSER_CONSENT_AUTO_OPEN_BROWSER=false` in headless environments. Confirm readiness with `auth status` before search calls.

### Search

```bash
tollbit auth status

tollbit search "climate policy"
tollbit search "climate policy" --size 10 --json
tollbit search "query" --properties example.com,cnn.com --programmatic-only
```

Environment:

- `TOLLBIT_GATEWAY_BASE_URL` — gateway base URL (default `https://gateway.tollbit.com`)

### Content

Price and fetch licensed publisher content:

```bash
tollbit content pricing https://example.com/article-1,https://example.com/article-2
tollbit content pricing https://example.com/article --json
tollbit content fetch https://example.com/article
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
tollbit content fetch https://example.com/article --confirm --json --rate-index 1
```

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost). Use `--toDisk=<path>` to save fetched content locally. When no user agent is configured, the org default `-tbcli-` agent is used. Set a registered user agent with `auth set --user-agent` or `--user-agent` on the fetch command.

### Guide

```bash
tollbit guide
tollbit guide --install ~/.claude/skills
```

Install writes `<dir>/<skill-name>/SKILL.md` (name from embedded frontmatter, e.g. `tollbit-cli`).

### Version

```bash
tollbit version
tollbit --version
```

Compare to the `version` field in the embedded skill when deciding whether to refresh an installed skill.

## Updating

Installer channel updates:

```bash
# Latest
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash

# Pinned
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.2.0 --force
```

```powershell
irm "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.ps1" | iex
Install-Tollbit -Version latest -Force
```

Other channels:

- npm: `npm update -g @tollbit/tollbit-cli`

## Assumptions

- Gateway API base URL defaults to `https://gateway.tollbit.com` (agent search).
- Agent identity tokens are minted via the auth service (`TOLLBIT_AUTH_BASE_URL`).
