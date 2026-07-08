# Tollbit CLI

Command-line client for [Tollbit](https://tollbit.com) agent identity, authorization, and publisher content search.

Binaries and installers for this distribution repo are published on **[Releases](https://github.com/tollbit/tollbit-cli-releases/releases)**.

## Install

#### Agent install prompt
**Agent?:** Install Tollbit from [github.com/tollbit/tollbit-cli-releases](https://github.com/tollbit/tollbit-cli-releases) using the install script or package that your environment needs (below), then run `tollbit guide --install <SKILLS_DIR>` to register the bundled skill.

### macOS and Linux

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash
```

Pin a version or choose an install directory:

```bash
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash -s -- --version vX.Y.Z
curl -fsSL "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.sh" | bash -s -- --install-dir "$HOME/bin" --force
```

By default the binary goes under `~/.local/bin`; the installer can add that directory to your shell `PATH`.

### Windows (PowerShell)

```powershell
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
```

Pin a version or skip `PATH` changes (useful in CI):

```powershell
irm "https://raw.githubusercontent.com/tollbit/tollbit-cli-releases/main/scripts/install.ps1" | iex
Install-Tollbit -Version vX.Y.Z -Force
Install-Tollbit -NoModifyPath -PrintPathInstructions
```

### npm

```bash
npm install -g @tollbit/tollbit-cli
```

Then run `tollbit` from any terminal (the package downloads the native binary for your OS).

### Manual install

From **[GitHub Releases](https://github.com/tollbit/tollbit-cli-releases/releases)**, download the archive for your OS and CPU, verify SHA-256 against `tollbit_<version>_checksums.txt`, and put `tollbit` (or `tollbit.exe`) on your `PATH`.

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
| **`guide`** | Print the agent guide; optionally install bundled skill markdown. |
| **`version`** | Print the CLI version string. |

Typical flow: **`auth login`** → **`auth status`** → **`search "query"`**. Use **`--help`** on each command for flags.

## Search examples

```bash
tollbit auth status

tollbit search "climate policy"
tollbit search "climate policy" --size 10 --json
tollbit search "query" --properties example.com,cnn.com --allowed-only
```

Environment:

- `TOLLBIT_GATEWAY_BASE_URL` — gateway base URL (default `https://gateway.tollbit.com`)
