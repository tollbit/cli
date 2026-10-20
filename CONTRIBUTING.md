# Contributing

The TollBit CLI is open source under the [MIT license](LICENSE), but it is
developed privately. This repository is a read-only mirror of each release:
every release is published here as a single commit and a matching `vX.Y.Z` tag,
so you can read and build the exact source of any version.

**We don't accept pull requests or issues here.** Pull requests are closed
automatically. For questions or problems, contact TollBit through
[tollbit.com](https://tollbit.com).

The rest of this file covers building and working on the CLI from source.

## Development

**CI:** `go test ./...` runs on pull requests and on pushes to `main` via [`.github/workflows/ci.yml`](.github/workflows/ci.yml) (Go version from [`go.mod`](go.mod)).

`tollbit guide` is an embedded copy of [`skill/tollbit-cli/SKILL.md`](skill/tollbit-cli/SKILL.md) — edit that file only; do not maintain a separate guide.

The repo includes a small `Makefile`:

| Target | What it runs |
|--------|----------------|
| `make test` | `go run gotest.tools/gotestsum@latest` (runs `go test ./...` with readable output) |
| `make build` | `go build -o tollbit ./cmd/tollbit` (binary at `./tollbit` in the repo root) |
| `make alias` | Prints a one-line `alias` so you can point `tollbit` at that binary |
| `make dev-install` | Builds and installs the repo binary into your `tollbit` command path, moving any existing installed binary to `tollbit-original` |
| `make dev-uninstall` | Restores `tollbit-original` back to `tollbit` and removes the dev-installed binary |
| `make bump VERSION=…` | Bumps `internal/version`, `skill/tollbit-cli/SKILL.md`, and install examples in [README.md](README.md) |

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

Before each command the CLI checks the version policy at `<foundry.base_url>/cli/v1/version-policy` and caches it in `update-check.json` in the runtime state dir. It warns on stderr when a newer version exists and stops below the supported minimum. `update.check_enabled` turns the check on; it is set in the config files only and cannot be overridden. `tb-cli.config.development.yaml` turns it off, so set it to `true` there to try the check locally. `update.check_interval` (`TOLLBIT_UPDATE_CHECK_INTERVAL`, e.g. `1h`) sets how often it refreshes. The env var works in release builds. `foundry.base_url` (`TOLLBIT_FOUNDRY_BASE_URL`) can be overridden in dev builds only.

When the check finds the CLI below the minimum, a build installed by `scripts/install.sh` updates itself when `update.auto_update` is on (`TOLLBIT_UPDATE_AUTO_UPDATE`, works in release builds): it downloads the release archive and checksums from `<update.release_base_url>/v<version>/`, verifies the SHA-256, replaces the binary and runs the command again. A global npm install updates through npm instead: `npm install -g @tollbit/tollbit-cli@<version>`, with npm's output on stderr, then the command runs again. It prints the instruction and exits 1 instead when `npm` is not on `PATH`, the binary is not under `<npm prefix -g>/lib/node_modules/@tollbit/tollbit-cli` (a project `node_modules`, `npx`, pnpm, Yarn or Volta), that folder or `<prefix>/bin` is not writable, on Windows (npm cannot replace the running binary), or when `npm install` fails. A gateway `426 cli_update_required` response does not update or re-run the command: the command fails with the update instruction, and the versions from the response are saved to `update-check.json` so the next command's check updates. `update.release_base_url` (`TOLLBIT_UPDATE_RELEASE_BASE_URL`) can be overridden in dev builds only. `tb-cli.config.development.yaml` turns `auto_update` off so a dev build never replaces itself; preview and `go install` builds, and binaries in a directory the user cannot write, print the update instruction and exit 1 instead.
