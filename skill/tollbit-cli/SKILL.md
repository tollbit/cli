---
name: tollbit-cli
version: 0.2.1
description: Search and ground content for agents on the TollBit network — search, price, and fetch with the tollbit CLI.
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
```

Results are labeled **Programmatic** (licensable via the CLI now) or **Enterprise** (reach out to Tollbit for access). Use `--programmatic-only` to limit results to Programmatic content. Without it, search spans the full catalog of discoverable content on the network.

Human (non-`--json`) search and pricing output ends with a leading command hint for the next step (pricing after search, fetch after pricing).

## Fetch (paid)

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost).

```bash
# Interactive: shows price, prompts to confirm, then a stderr "Fetching" spinner until the body prints to stdout
tollbit content fetch https://example.com/article

# Non-interactive automation (still paid)
tollbit content fetch https://example.com/article --confirm --user-agent MyAgent-User

# Save content to disk
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md

# Machine-readable output
tollbit content fetch https://example.com/article --confirm --json
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

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.
