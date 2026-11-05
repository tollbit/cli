# TollBit CLI

CLI client for searching and grounding content for agents, allows access to all content in the [TollBit](https://tollbit.com) network.

Binaries and installers are published on **[GitHub Releases](https://github.com/tollbit/cli/releases)**.

Primary workflow: install → `search` → `content pricing` → `content fetch`. Authentication is triggered automatically by the CLI when needed.

## Install

#### Agent install prompt

**Agent?:** Install TollBit from [github.com/tollbit/cli](https://github.com/tollbit/cli) using the install script or package that your environment needs (below), then run `tollbit guide --install <SKILLS_DIR>` to register the bundled skill (add `--skill analytics` for the analytics skill).

### macOS and Linux

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash
```

Pin a version or choose an install directory:

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.5.0
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
Install-Tollbit -Version v0.5.0 -Force
Install-Tollbit -NoModifyPath -PrintPathInstructions
```

### npm

```bash
npm install -g @tollbit/tollbit-cli
```

Then run `tollbit` from any terminal (the package downloads the native binary for your OS).

### Claude plugin

This repo is a Claude plugin marketplace. The `tollbit` plugin adds the agent skills (what `tollbit guide --skill cli` and `--skill analytics` print), a skill that installs and sets up the CLI in Claude's sandbox (chat and Cowork), and a skill that diagnoses failed installs. In Claude Code:

```
/plugin marketplace add tollbit/cli
/plugin install tollbit@tollbit
```

In the Claude desktop or web app (chat and Cowork), go to **Customize → Plugins → Add → Add marketplace**, enter `tollbit/cli`, then install **tollbit**.

On a Team or Enterprise plan, an organization owner must turn on **Skills**, **User-created skills** and **Cowork**; otherwise **Add marketplace** doesn't appear. To install the CLI in Claude's sandbox, **Code execution** must also allow package managers and GitHub, plus `oauth.tollbit.com` and `gateway.tollbit.com`.

### Manual install

From **[GitHub Releases](https://github.com/tollbit/cli/releases)**, download the archive for your OS and CPU, verify SHA-256 against `tollbit_<version>_checksums.txt`, and put `tollbit` (or `tollbit.exe`) on your `PATH`.

### If the install fails

Paste the prompt in [prompts/install-troubleshooting.md](prompts/install-troubleshooting.md) into an AI agent on the computer where the install failed (Claude Code, Codex, Claude in the desktop app, ...). It finds where the install breaks, such as a blocked host, a proxy, missing write access or a blocked binary, and writes a report to send to TollBit support. The Claude plugin includes the same instructions as a skill.

### Build from source

```bash
go build ./cmd/tollbit
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for local development and release instructions.

## For AI coding agents

- Install `tollbit` so it is on **`PATH`** in every environment where you run shell commands (CI images, local sandboxes, agent runners).
- Run **`tollbit guide`** for orientation, then **`tollbit guide --install <SKILLS_DIR>`** to persist the bundled skill. Use **`--skill analytics`** for the skill that queries your organization's site analytics. The guide is the full automation contract (exit codes, streams, non-interactive fetch).
- Typical flow: **`search`** → **`content pricing`** → **`content fetch`**. The CLI prompts for authentication when a token is required.
- Optionally set an agent profile with **`tollbit auth set --name <name>`** (or `TOLLBIT_AGENT_DEFAULT_NAME`) / **`--user-agent`**.
- Prefer **`--json`** on **`search`**, **`content pricing`**, **`content fetch`**, **`feedback`**, and **`auth status`**. Exit codes: `0` success, `1` runtime, `2` usage; stdout is data-only (hints and errors on stderr).

## What the CLI can do

| Command | Purpose |
|--------|---------|
| **`search "query"`** | Search publisher content via the gateway API. |
| **`content pricing/fetch`** | Price and fetch licensed publisher content. |
| **`feedback "message"`** | Submit CLI / agent feedback (requires OBO). |
| **`auth login/logout/status/set`** | Agent profile and OAuth authorization token (also run automatically when needed). |
| **`guide`** | Print an agent skill (`--skill cli`, the default, or `--skill analytics`); optionally install it. |
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

### Feedback

Submit product feedback (requires an OBO-linked agent session):

```bash
tollbit feedback "search ranking felt off for this query"
tollbit feedback "great fetch UX" --rating 5 --category content
tollbit feedback "auth hung" --rating 2 --category auth --metadata source=cli --json
```

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
tollbit guide --skill analytics --install ~/.claude/skills
```

Install writes `<dir>/<skill-name>/SKILL.md` (name from embedded frontmatter, e.g. `tollbit-cli`).

### Doctor

```bash
tollbit doctor
tollbit doctor --json
```

Checks that the CLI can work on this machine, and is the first thing to run when it installs but does not work:

- **Execution:** version, OS/arch, install method, the binary's path, and whether `tollbit` on PATH is this binary.
- **Directories:** that the CLI can create and write its state (`TOLLBIT_RUNTIME_STATE_DIR`) and credentials (`TOLLBIT_CREDENTIALS_STORAGE_DIR`) directories, and whether it can update itself in its install directory.
- **Network:** proxy environment variables (credentials redacted), and for each host the CLI is configured to call (auth, gateway, and GitHub releases, following the redirect to the asset host) whether it resolves, completes TLS and answers. Failures name their kind: `dns`, `connect`, `timeout`, `proxy_unreachable`, `proxy_rejected`, `proxy_auth_required`, `tls_intercepted` (with the certificate issuer), `tls_other`, or `http_<status>`. On Windows it warns when a system proxy is set but `HTTPS_PROXY` is not, since the CLI only uses the environment variables.
- **Auth:** whether a token is stored and when it expires, read from disk only. Informational; it never fails.
- **Skills:** the agent skills bundled with this CLI (`tollbit-cli`, `tollbit-analytics`) and their version, with the command to install or update each. Doctor does not look for installed copies, since every agent keeps skills in its own place; an agent compares the `version:` line of the SKILL.md it loaded. Informational; it never fails.

Each line is `PASS`, `WARN`, `FAIL` or `INFO`, with a hint for anything not passing. Exits `1` when any check fails. Never prints tokens or credentials. `doctor` skips the automatic update check, so it runs even when an update is required.

### Version

```bash
tollbit version
tollbit --version
```

Compare to the `version` field in the embedded skill when deciding whether to refresh an installed skill.

## Updating

### Automatic updates

About once an hour the CLI checks which versions TollBit supports. When a newer version exists, it prints a notice on stderr. When this version is below the supported minimum, it installs the update itself, says so on stderr, and runs the command again. Script installs replace the binary; global npm installs run `npm install -g @tollbit/tollbit-cli@<version>`. Other installs (`go install`, preview builds, pnpm, Yarn, Volta, `npx`, npm on Windows, or a folder that needs `sudo`) print the update command and exit `1`.

`TOLLBIT_UPDATE_AUTO_UPDATE=false` stops the self-install only. The CLI still warns below the latest version and still stops with exit `1` below the minimum.

### Updating by hand

Installer channel updates:

```bash
# Latest
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash

# Pinned
curl -fsSL "https://raw.githubusercontent.com/tollbit/cli/main/scripts/install.sh" | bash -s -- --version v0.5.0 --force
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

## Source and contributions

The CLI is developed privately and published here release by release. Each
release is a single commit on `main` with a matching `vX.Y.Z` tag, and the
[release workflow](.github/workflows/release.yml) builds the binaries on
[GitHub Releases](https://github.com/tollbit/cli/releases) from exactly that
source. We don't accept pull requests or issues here; see
[CONTRIBUTING.md](CONTRIBUTING.md).

To check that a release archive was built from this repository by that workflow:

```bash
gh attestation verify tollbit_<version>_<os>_<arch>.tar.gz --repo tollbit/cli
```

To read or build the exact source of a release, check out its tag
(`git checkout v<version>`).

## License

MIT — see [LICENSE](LICENSE). Binary distributions include third-party license notices in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).
