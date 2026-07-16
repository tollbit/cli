---
name: tollbit-cli
version: 0.2.2
description: Search for news and articles and ground answers in licensed publisher content on the TollBit network. Use whenever the user wants to find news, articles, reporting, or sources on a topic or current event — searches the catalog, then prices and fetches full article content (paid) with the tollbit CLI.
---

# Tollbit CLI

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

Results are labeled **Programmatic** (licensable via the CLI now) or **Enterprise** (reach out to Tollbit for access). Use `--programmatic-only` to limit results to Programmatic content. Without it, search spans the full catalog of discoverable content on the network. Only Programmatic results can be priced and fetched via the CLI.

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

## For automation

- Prefer `--json` for machine-readable output on `search`, `content pricing`, `content fetch`, and `auth status`.
- **Exit codes:** `0` success · `1` runtime error · `2` usage error. `auth status --check` → `0` valid · `1` invalid/expired · `2` missing. Before paid `fetch`, prefer `auth status --check` (or `auth status --json`) so you fail fast instead of hanging on interactive consent.
- **Streams:** stdout carries data only; prompts, spinners, next-step hints, and errors go to stderr. Parse stdout for success data; treat non-zero exit as failure and read stderr when diagnosing.
- **Non-interactive fetch:** never call `content fetch` without `--confirm`. Pass `--rate-index N` when multiple rates exist (required with `--json` in that case). Every fetch still charges.
- **Licensable results:** only **Programmatic** results can be priced and fetched; use `--programmatic-only` or skip Enterprise hits.
- **Pagination:** reuse the `--next-token` / `nextToken` from the previous search response when more results exist.

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.
