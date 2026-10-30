---
name: tollbit-analytics
version: {{ .Version }}
description: Query TollBit analytics for the sites of the organization the agent works for — which bots, crawlers and AI agents visit, how often, which pages they request, referrer traffic, and extracted entities and brands. Use whenever the user asks about traffic to their own sites from bots or agents, crawler activity, or TollBit analytics, with the tollbit CLI's read-only SQL.
---

# TollBit analytics

`tollbit analytics` runs read-only SQL against TollBit analytics for the sites your operator's organization owns. Use it to answer questions such as which AI crawlers visit a site, how often, and which pages they request.

Always work in this order: `schema` → `query`.

```bash
tollbit analytics schema
tollbit analytics query "SELECT user_agent, SUM(count) AS requests FROM user_agent_aggregate WHERE timestamp >= '2026-09-04' GROUP BY user_agent ORDER BY requests DESC LIMIT 20"
```

Both commands print JSON to stdout and errors to stderr. They need an authorized agent token; the CLI starts authorization when it needs one (see the tollbit-cli skill's Auth section).

## Schema first

Run `tollbit analytics schema` once per session, before your first query. Run it again if a query fails on an unknown table or a limit. The tables, columns and limits depend on the organization's access and can change on the server, so don't rely on what you saw in an earlier session. It reports:

- `dialect`: the SQL dialect to write.
- `tables`: the tables this organization can query, with their columns. Each table has a description and a `clustering` order; each column has a type and a description. Query only the tables and columns listed here.
- `partition_column` and `require_partition_filter`: which date filter a table needs.
- `limits`: the server's limits (rows, bytes scanned, time) with their values and units. Never assume a limit; read it here.

## Writing queries

- Send exactly one `SELECT` statement, quoted as a single argument. `SHOW`, `DESCRIBE` and statements that change data are rejected.
- Always filter on `timestamp` (or `day`) to limit the scan. `LIMIT` alone does not reduce the bytes scanned.
- Log tables hold daily counts: their `timestamp` is a day bucket at 00:00 UTC, so filter and group by whole days.
- Entity tables hold extraction records: their `timestamp` is the extraction time. Use `UNNEST` for repeated fields.
- Daily brand traffic needs a `day` filter. Select one `level`, and tell brand rows apart from scope totals (`entity_name = ''`).
- Daily page counts don't add up to an exact count of distinct pages over several days.
- Intelligence tables need publisher intelligence access as well as SQL access.

Examples (replace the dates):

```bash
# Requests per user agent since a date
tollbit analytics query "SELECT user_agent, SUM(count) AS requests FROM user_agent_aggregate WHERE timestamp >= '2026-09-04' GROUP BY user_agent ORDER BY requests DESC LIMIT 20"

# Most requested paths on one host
tollbit analytics query "SELECT path, SUM(count) AS requests FROM agent_logs_by_page WHERE host = 'example.com' AND timestamp >= '2026-09-04' GROUP BY path ORDER BY requests DESC LIMIT 20"

# Daily trend for one crawler
tollbit analytics query "SELECT timestamp, SUM(count) AS requests FROM user_agent_aggregate WHERE user_agent = 'GPTBot' AND timestamp >= '2026-08-12' GROUP BY timestamp ORDER BY timestamp"
```

## Reading results

A query prints a JSON object:

- `columns`: each column's `name` and `type`.
- `rows`: arrays in column order; `null` for missing values.
- `meta`: `row_count`, `truncated` and `duration_ms`.

When `meta.truncated` is `true`, the result was cut at the server's row limit and a warning is printed to stderr. Don't treat a truncated result as complete: add `ORDER BY` with `LIMIT` and `OFFSET` to page through it, or narrow the query.

## For automation

- **Exit codes:** `0` success · `1` runtime error (including a rejected query) · `2` usage error · `3` authorization pending.
- **Streams:** stdout carries only the JSON result; hints, warnings and errors go to stderr. When a query fails, read stderr: for known errors the CLI prints a next step, such as running `analytics schema` for an unknown table.
- Report numbers from the result itself, and say which date range and filters produced them.

Install this skill: `tollbit guide --skill analytics --install <SKILLS_DIR>`.
