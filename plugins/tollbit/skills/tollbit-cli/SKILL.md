---
name: tollbit-cli
version: 0.4.3
description: Search for news and articles and ground answers in licensed publisher content on the TollBit network. Use whenever the user wants to find news, articles, reporting, or sources on a topic or current event — searches the catalog, then prices and fetches full article content (paid) with the tollbit CLI.
---

# TollBit CLI

Primary workflow: `search` → `content pricing` → `content fetch`. Authentication is triggered automatically by the CLI when needed.

```bash
tollbit search "climate policy" --size 10
tollbit content pricing https://example.com/article-1,https://example.com/article-2
tollbit content pricing https://example.com/article --json
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
```

## Search

```bash
tollbit search "climate policy" --size 10
tollbit search "query" --json
tollbit search "query" --programmatic-only
tollbit search "query" --next-token "…"
```

Results are labeled **Programmatic** (licensable via the CLI now) or **Enterprise** (reach out to TollBit for access). Use `--programmatic-only` to limit results to Programmatic content. Without it, search spans the full catalog of discoverable content on the network. Only Programmatic results can be priced and fetched via the CLI.

When more results exist, human output prints a `--next-token` value (or `nextToken` in `--json`); pass it back to continue.

Human (non-`--json`) search and pricing write a next-step command hint to stderr (pricing after search, fetch after pricing).

## Fetch (paid)

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost).

```bash
# Interactive: shows price, prompts to confirm, then a stderr "Fetching" spinner until the body prints to stdout
tollbit content fetch https://example.com/article

# Non-interactive automation (still paid)
tollbit content fetch https://example.com/article --confirm --user-agent MyAgent-User

# Multi-rate / --json: pick a rate explicitly
tollbit content fetch https://example.com/article --confirm --json --rate-index 1

# Save content to disk
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
```

Use `--toDisk=<path>` to persist fetched content locally. With `--json`, the full API response is written to stdout (and to disk when `--toDisk` is set).

When no user agent is configured, the org default `-tbcli-` agent is used. Set one with `auth set --user-agent` or pass `--user-agent` on fetch. If the user agent is not registered, the CLI prints the API error and registration guidance (no interactive picker).

## Auth (optional)

Auth runs automatically when a command needs a token. To manage the profile explicitly:

```bash
tollbit auth login --name my-agent --user-agent MyAgent-User
tollbit auth status
tollbit auth set --name my-agent --user-agent MyAgent-User
```

End-user proximity describes where the agent environment running the CLI is located relative to the end user's browser. The CLI uses that context to present either the local or the detached (remote) authorization flow. Follow the flow the CLI presents.

Configured local flow: local browser

- For local authorization, run `tollbit auth login` in the same environment as the end user's browser. The CLI opens the browser when configured to do so, waits for the loopback callback, and completes authorization automatically.
- No detached completion command is used for the local browser flow; if authorization is interrupted, rerun `tollbit auth login`.

Configured remote flow: detached icon confirmation

- For remote authorization, run `tollbit auth login` and relay only the printed consent URL to the end user. Do not send the valid icon vocabulary to the user; it is for the agent's spelling reference only.
- After the end user authorizes in their browser, they will read back three icon names. Verify the icon names against the list of valid icons previously provided in `auth login`. Then run `tollbit auth complete <first> <second> <third>` using those names in exactly the displayed order.

Remote troubleshooting:
- `unrecognized icon name`: one submitted word did not match the icon manifest and no attempt was consumed; confirm spelling with the end user using the printed vocabulary, then rerun `tollbit auth complete <first> <second> <third>`.
- `authorization denied`: the icons or order were wrong and the challenge is dead; restart with `tollbit auth login`.
- `authorization still pending`: the end user has not clicked Authorize yet; wait for the end user to finish, then rerun `tollbit auth complete <first> <second> <third>`.

If `auth login` or `auth complete` reports that authorization is still pending, it exits with code 3 and keeps the pending authorization. Wait for the end user to finish and run `auth complete` again (`auth complete` takes: pending auth plus three icon names). After authorization succeeds, retry the command that required user/org authorization. Use `tollbit runtime status` to inspect the current setting. When the agent environment is known, prefer `tollbit runtime set --end-user-proximity local|remote` so future commands use the right flow automatically. Use `--end-user-proximity local|remote` only for a one-off override.

## For automation

- Prefer `--json` for machine-readable output on `search`, `content pricing`, `content fetch`, `feedback`, and `auth status`.
- **Exit codes:** `0` success · `1` runtime error · `2` usage error · `3` authorization pending (`auth login` / `auth complete` in a detached flow). `auth status --check` → `0` valid · `1` invalid/expired · `2` missing. Before paid `fetch`, prefer `auth status --check` (or `auth status --json`) so you fail fast instead of hanging on interactive consent.
- **Streams:** stdout carries data only; prompts, spinners, next-step hints, and errors go to stderr. Parse stdout for success data; treat non-zero exit as failure and read stderr when diagnosing.
- **Updates:** the CLI may update itself, print a notice on stderr, and run the command again. stdout and the exit code are the command's own; there is nothing to do. `TOLLBIT_UPDATE_AUTO_UPDATE=false` stops the self-install only: the CLI still warns on stderr when a newer version exists, and still exits `1` with the update command below the supported minimum.
- **Non-interactive fetch:** never call `content fetch` without `--confirm`. Pass `--rate-index N` when multiple rates exist (required with `--json` in that case). Every fetch still charges.
- **Licensable results:** only **Programmatic** results can be priced and fetched; use `--programmatic-only` or skip Enterprise hits.
- **Pagination:** reuse the `--next-token` / `nextToken` from the previous search response when more results exist.
- **Feedback:** use `tollbit feedback "…"` (optional `--rating`, `--category`, `--metadata key=value`) when the user wants to report CLI or product issues to TollBit.

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.

Auth instructions rendered for local=redirect, remote=agent_confirms_icons; re-run `tollbit guide --install <SKILLS_DIR>` if the CLI's auth configuration changes.
