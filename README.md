# Tollbit CLI

CLI client for searching and grounding content for agents, allows access to all content in the [TollBit](https://tollbit.com) network.

Binaries and installers are published on **[GitHub Releases](https://github.com/tollbit/cli/releases)**.

Primary workflow: install → `search` → `content pricing` → `content fetch`. Authentication is triggered automatically by the CLI when needed.

## Install

#### Agent install prompt

**Agent?:** Install Tollbit from [github.com/tollbit/cli](https://github.com/tollbit/cli) using the install script or package that your environment needs (below), then run `tollbit guide --install <SKILLS_DIR>` to register the bundled skill.

### macOS and Linux

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash
```

Pin a version or choose an install directory:

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.2.2
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
Install-Tollbit -Version v0.2.2 -Force
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

## For AI coding agents

- Install `tollbit` so it is on **`PATH`** in every environment where you run shell commands (CI images, local sandboxes, agent runners).
- Run **`tollbit guide`** for orientation, then **`tollbit guide --install <SKILLS_DIR>`** to persist the bundled skill. The guide is the full automation contract (exit codes, streams, non-interactive fetch).
- Typical flow: **`search`** → **`content pricing`** → **`content fetch`**. The CLI prompts for authentication when a token is required.
- Optionally set an agent profile with **`tollbit auth set --name <name>`** (or `TOLLBIT_AGENT_DEFAULT_NAME`) / **`--user-agent`**.
- Prefer **`--json`** on **`search`**, **`content pricing`**, **`content fetch`**, and **`auth status`**. Exit codes: `0` success, `1` runtime, `2` usage; stdout is data-only (hints and errors on stderr).

## What the CLI can do

| Command | Purpose |
|--------|---------|
| **`search "query"`** | Search publisher content via the gateway API. |
| **`content pricing/fetch`** | Price and fetch licensed publisher content. |
| **`auth login/logout/status/set`** | Agent profile and OAuth authorization token (also run automatically when needed). |
| **`guide`** | Print the agent guide; optionally install bundled skill markdown. |
| **`version`** | Print the CLI version string. |

Typical flow: **`search "query"`** → **`content pricing <url>`** → **`content fetch <url>`**. Use **`--help`** on each command for flags.

## Commands

### Search

Search first; the CLI will prompt for authentication if your session is not ready yet.

```bash
tollbit search "climate policy"
tollbit search "climate policy" --size 10 --json
tollbit search "query" --properties example.com,cnn.com --programmatic-only
```

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

### Auth

Auth runs automatically when a command needs a token. Use these commands to inspect or manage the profile explicitly:

```bash
tollbit auth login --name my-agent --user-agent MyAgent-User
tollbit auth status
tollbit auth status --json
tollbit auth status --check
tollbit auth set --name my-agent --user-agent MyAgent-User
tollbit auth logout
tollbit auth logout --all
```

Logging in again (`tollbit auth login`) mints a new token and revokes all previous ones — use it if you believe a token was exposed. `auth logout` revokes the current token; `auth logout --force` clears local credentials even if the server can't be reached (the token is then revoked at your next login or expires within 30 days).

`TOLLBIT_AGENT_DEFAULT_NAME` and `TOLLBIT_AGENT_DEFAULT_USER_AGENT` set fallback profile defaults. Saved profile overrides those defaults. `search` and `content` accept `--user-agent` as a per-request override.

`auth status --check` exits `0` when the token is valid, `1` when invalid/expired, and `2` when missing (no stdout).

Set `TOLLBIT_AUTH_BROWSER_CONSENT_AUTO_OPEN_BROWSER=false` in headless environments.

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
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.2.2 --force
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

## Configure

Agent credentials are stored under `TOLLBIT_CREDENTIALS_STORAGE_DIR` (default platform path): `agent-identity.json` and `agent-token.jwt`.

## License

MIT — see [LICENSE](LICENSE). Binary distributions include third-party license notices in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).
