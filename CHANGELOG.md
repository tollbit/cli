# Changelog

All notable changes to the TollBit CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.8] - 2026-09-28

### Changed

- Help text, messages and documentation now use the TollBit brand name
  consistently.

## [0.3.7] - 2026-09-28

### Changed

- Documentation and repository housekeeping.

## [0.3.6] - 2026-09-28

### Added

- This changelog. GitHub Release notes now come from it.

### Changed

- This repository is now a read-only mirror of released source: each release is
  published as a single commit and tag. Pull requests and issues are no longer
  accepted; see [CONTRIBUTING.md](CONTRIBUTING.md).

## [0.3.5] - 2026-09-15

### Changed

- `tollbit analytics query --help` now describes the SQL dialect as standard
  SQL (SQL:2011), matching the `dialect` field reported by
  `tollbit analytics schema`.

## [0.3.4] - 2026-09-15

### Added

- `analytics query` output includes a `meta` object (row count, whether the
  result was truncated, bytes scanned, and duration) when the server reports
  it, and warns on stderr when the result was cut off at the server's row
  limit.
- `analytics query` suggests a next step when a query fails because of an
  unknown table, a statement that is not allowed, the scan limit, or a
  timeout.

### Changed

- `analytics schema` now prints a JSON object with `dialect`, `tables`, and
  `limits` instead of a bare array of tables. Tables and columns can include
  descriptions, allowed values, and clustering columns. Update scripts that
  parse the old array.

## [0.3.3] - 2026-09-11

### Changed

- `tollbit analytics` help now explains the SQL dialect, that only a single
  `SELECT` statement is accepted, how daily timestamp buckets work, and how to
  stay within the server's scan and row limits, with working examples.
- `analytics query` reports clearer usage errors for empty SQL or more than
  one argument.

### Removed

- The `--user-agent` flag on `analytics query` and `analytics schema`. It had
  no effect on analytics requests.

## [0.3.2] - 2026-09-11

### Added

- `tollbit analytics schema` lists the analytics tables and columns you can
  query, as JSON.

## [0.3.1] - 2026-09-11

### Changed

- Internal fixes; no user-facing changes.

## [0.3.0] - 2026-09-11

### Added

- `tollbit analytics query <SQL>` runs a SQL query against your
  organization's TollBit analytics and prints the result as JSON. It requires
  authorization on behalf of a user and organization.

### Changed

- `auth status`, `auth login`, and `auth complete` show the email address and
  organization name you are authorized on behalf of, not just their IDs.
  `auth status --json` adds `primary_email` and `organization_name`. If the
  names cannot be looked up, the IDs are shown with a warning.
- `auth status` output is aligned in columns, with the on-behalf-of details on
  separate lines.

## [0.2.5] - 2026-08-06

### Added

- `tollbit feedback "message"` sends feedback about the CLI to TollBit. Add an
  optional `--rating` (1 to 5), `--category`, or repeatable
  `--metadata key=value`, and use `--json` for machine-readable output.

### Changed

- When end-user proximity has not been set, the CLI now uses the local browser
  login flow instead of the remote flow. To keep the remote flow, run
  `tollbit runtime set --end-user-proximity remote`.

## [0.2.4] - 2026-07-30

### Added

- A remote login flow for agents that run somewhere other than the user's
  browser, such as a cloud sandbox. `tollbit auth login` prints a consent URL
  for the user to open; after approving, the user reads back three icons shown
  on the page, and the agent finishes with
  `tollbit auth complete <first> <second> <third>`.
- `tollbit auth complete` finishes a pending authorization. `auth login` and
  `auth complete` exit with code 3 while authorization is still pending, and
  `auth status` shows the pending authorization.
- `tollbit runtime status`, `tollbit runtime set`, and `tollbit runtime help`
  view and save the end-user proximity: `local` when the CLI runs next to the
  user's browser, `remote` when it runs elsewhere. It decides which login flow
  is used. Override it for one run with the global `--end-user-proximity` flag
  or `TOLLBIT_RUNTIME_END_USER_PROXIMITY`. Without a saved setting, the remote
  flow is used.
- `tollbit auth login` prints the end-user proximity and login flow in use.
- `tollbit guide` and the installed skill include login instructions for the
  local and remote flows.

### Fixed

- `tollbit search --programmatic-only` now filters on whether content is ready
  to license, matching the Programmatic label shown in results.

## [0.2.3] - 2026-07-21

### Added

- `tollbit auth logout --force` clears local credentials even when the token
  cannot be revoked on the server.
- Install with `go install github.com/tollbit/cli/cmd/tollbit@latest`. Update
  notices for Go installs show the matching `go install` command.
- Release archives and the npm package include the MIT `LICENSE`. Release
  archives also include third-party license notices.

### Changed

- `tollbit auth logout` no longer removes local credentials when it cannot
  reach the server to revoke the token. It exits with an error and you stay
  logged in; retry when online, or use `--force`.

### Security

- Release binaries always connect to TollBit's official auth and API
  endpoints. Endpoint overrides from environment variables, flags, or config
  files are ignored.
- During login, the CLI only opens `http` and `https` URLs in the browser.

## [0.2.2] - 2026-07-16

### Changed

- The bundled agent skill has a clearer description, so agents use it more
  reliably when asked for news, articles, or sources on a topic. Reinstall it
  with `tollbit guide --install <SKILLS_DIR>`.

## [0.2.1] - 2026-07-15

### Changed

- Next-step hints after `search` and `content pricing` are now written to
  stderr, so stdout contains only results.
- `tollbit guide` and the bundled skill lead with the search, pricing, and
  fetch workflow, and document exit codes, output streams, and non-interactive
  fetch for automation.

### Security

- The CLI no longer loads a `.env` file from the current directory
  automatically. Set `TOLLBIT_ENV_FILE` to load one; only `TOLLBIT_`-prefixed
  variables are read from it.

## [0.2.0] - 2026-07-15

### Added

- `tollbit search "query"` searches content on the TollBit network. Results
  are labeled Programmatic (licensable through the CLI) or Enterprise (contact
  TollBit for access). Supports `--size`, `--next-token`, `--properties`,
  `--programmatic-only`, `--user-agent`, and `--json`.
- `tollbit content pricing <url>[,<url>...]` shows licensing rates for one or
  more article URLs.
- `tollbit content fetch <url>` buys and fetches licensed content. The price
  is shown for confirmation unless you pass `--confirm`; every fetch is
  charged. Use `--toDisk <path>` to save the content, `--rate-index` to pick a
  rate, and `--json` for the full response. A spinner shows progress.
- `tollbit auth` commands for the agent profile and authorization: `login`,
  `logout` (`--all` also clears the profile), `set`, and `status` (`--json`,
  and `--check` for scripts: exit 0 when valid, 1 when invalid or expired, 2
  when missing).
- Commands that need authorization start the browser login automatically.
  Refresh tokens keep you logged in after the access token expires.
- `search` and `content pricing` suggest the next command to run.
- `tollbit guide` prints the agent guide, and
  `tollbit guide --install <SKILLS_DIR>` installs it as an agent skill.
- `tollbit version` and `tollbit --version`.
- Install with the shell script (macOS and Linux), the PowerShell script
  (Windows), npm (`@tollbit/tollbit-cli`), or the archives on GitHub Releases.
- The CLI warns on stderr when a newer version is available and shows the
  update command for how it was installed. Versions that are no longer
  supported get a clear "update required" message.

### Changed

- Earlier versions (0.1.x and before) were released from a previous repository.

[0.3.8]: https://github.com/tollbit/cli/compare/v0.3.7...v0.3.8
[0.3.7]: https://github.com/tollbit/cli/compare/v0.3.6...v0.3.7
[0.3.6]: https://github.com/tollbit/cli/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/tollbit/cli/compare/v0.3.4...v0.3.5
[0.3.4]: https://github.com/tollbit/cli/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/tollbit/cli/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/tollbit/cli/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/tollbit/cli/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/tollbit/cli/compare/v0.2.5...v0.3.0
[0.2.5]: https://github.com/tollbit/cli/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/tollbit/cli/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/tollbit/cli/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/tollbit/cli/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/tollbit/cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/tollbit/cli/releases/tag/v0.2.0
