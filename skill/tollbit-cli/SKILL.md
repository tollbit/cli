---
name: tollbit-cli
version: 0.0.1
description: Manage Tollbit agent identity, authorization, content search, and pricing with the tollbit CLI.
---

# Tollbit CLI

Configure agent identity and authorization:

```bash
tollbit identity set my-agent
tollbit identity get
tollbit agent status
tollbit agent login
```

Search publisher content:

```bash
tollbit search "climate policy" --size 10
tollbit search "query" --json
```

Content workflow (search → pricing → fetch):

```bash
tollbit search "climate policy" --size 10
tollbit pricing https://example.com/article-1,https://example.com/article-2
tollbit content pricing https://example.com/article --json
```

Install this skill: `tollbit guide --install <SKILLS_DIR>`.
Compare frontmatter `version` with `tollbit version` when updating.
