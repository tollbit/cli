# Contributing

The Tollbit CLI is open source under the [MIT license](LICENSE), but it is
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
