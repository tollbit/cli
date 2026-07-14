---
name: tollbit-cli
version: 0.0.1
description: Manage Tollbit agent profile, authorization, content search, pricing, and paid content fetch with the tollbit CLI.
---

# Tollbit CLI

Configure agent profile and authorization:

```bash
tollbit auth login --name my-agent --user-agent MyAgent-User
tollbit auth status
```

Search publisher content:

```bash
tollbit search "climate policy" --size 10
tollbit search "query" --json
tollbit search "query" --programmatic-only
```

Results are labeled **Programmatic** (licensable via the CLI now) or **Enterprise** (reach out to Tollbit for access). Use `--programmatic-only` to limit results to Programmatic content. Without it, search spans the full catalog of discoverable content on the network.

Content workflow (search → pricing → fetch):

```bash
tollbit search "climate policy" --size 10
tollbit content pricing https://example.com/article-1,https://example.com/article-2
tollbit content pricing https://example.com/article --json
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
```

Human (non-`--json`) search and pricing output ends with a leading command hint for the next step (pricing after search, fetch after pricing).
## Fetch (paid)

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost).

```bash
# Interactive: shows price, prompts to confirm, prints article body to stdout
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

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.
