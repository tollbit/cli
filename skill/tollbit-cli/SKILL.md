---
name: tollbit-cli
version: 0.0.1
description: Manage Tollbit agent identity, authorization, content search, pricing, and paid content fetch with the tollbit CLI.
---

# Tollbit CLI

Configure agent identity and authorization:

```bash
tollbit identity set my-agent --user-agent MyAgent-User
tollbit identity get
tollbit agent status
tollbit agent login
```

Search publisher content:

```bash
tollbit search "climate policy" --size 10
tollbit search "query" --json
tollbit search "query" --allowed-only
```

Use `--allowed-only` to limit results to publisher properties your account has access to. Without it, search spans the full catalog of discoverable content on the network.

Content workflow (search → pricing → fetch):

```bash
tollbit search "climate policy" --size 10
tollbit content pricing https://example.com/article-1,https://example.com/article-2
tollbit content pricing https://example.com/article --json
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md
```

## Fetch (paid)

**Every fetch charges money.** Pricing is shown and you must confirm unless you pass `--confirm` (automation still incurs cost).

```bash
# Interactive: shows price, prompts to confirm, prints article body to stdout
tollbit content fetch https://example.com/article

# Non-interactive automation (still paid)
tollbit content fetch https://example.com/article --confirm --agent-user-agent MyAgent-User

# Save content to disk
tollbit content fetch https://example.com/article --confirm --toDisk ./article.md

# Machine-readable output
tollbit content fetch https://example.com/article --confirm --json
```

Use `--toDisk=<path>` to persist fetched content locally. With `--json`, the full API response is written to stdout (and to disk when `--toDisk` is set).

If the configured user agent is not registered, the CLI lists available user agents, prompts you to pick one, and saves it to identity for future fetches.

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.
