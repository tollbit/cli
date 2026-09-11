# `tollbit analytics` usability fixes

## Context

A help-only review of `tollbit analytics` (findings in `analytics-cli-review.md`, untracked) turned up 31 issues. The command group is agent-facing, so the fixes split into three buckets: what the CLI itself can fix (help text, output, input, error hints), what the bundled skill must teach agents, and what only the server can provide (limits, truncation, descriptions, error codes, data-model cleanup). Server limits (10k rows, ~30 GB scanned, 10 s) are configured server-side and must never be hard-coded in the CLI; the CLI renders whatever the server reports.

Decisions already made:
- JSON stays the default output. Add other formats behind a flag.
- Server limits come from the server, not the CLI. Proposed contract below.
- `--user-agent` is removed from both analytics commands. It only affected token minting and never reached the analytics request. The client still sends the CLI's own `User-Agent` and `X-Tollbit-Client` headers.
- Server data-model asks are recorded as a backlog section, not built here.

## Proposed server contract (for the backlog; CLI tolerates old and new shapes)

**Schema `GET /analytics/agent/v1/query/schema`** moves from a top-level array to an object:

```json
{
  "dialect": "bigquery",
  "tables": [
    {
      "name": "user_agent_aggregate",
      "description": "Daily request counts per host, user agent, status class and request kind.",
      "columns": [
        {"name": "timestamp", "type": "TIMESTAMP", "description": "Day bucket, 00:00 UTC."},
        {"name": "type", "type": "STRING", "description": "Request kind.", "values": ["REQUEST", "ROBOT", "SITEMAP", "WELL-KNOWN"]}
      ]
    }
  ],
  "limits": {
    "max_rows":          {"value": 10000,       "unit": "rows",    "description": "Result sets are capped at this many rows. Use ORDER BY with LIMIT/OFFSET to page."},
    "max_bytes_scanned": {"value": 32212254720, "unit": "bytes",   "description": "Queries estimated to scan more than this are rejected. Filter on timestamp and select fewer columns."},
    "max_duration":      {"value": 10,          "unit": "seconds", "description": "Queries running longer than this are cancelled."}
  }
}
```

`limits` is a map of name to `{value, unit, description}`. The CLI iterates the map and prints every entry it receives, so new limits need no CLI change. `description` and `values` are optional everywhere; the CLI omits what is absent.

**Query `POST /analytics/agent/v1/query`** adds a `meta` block:

```json
{"columns": [...], "rows": [...], "meta": {"row_count": 10000, "truncated": true, "bytes_scanned": 1234567, "duration_ms": 850}}
```

**Errors** keep ProblemJSON and add `code` values the CLI can map to hints: `analytics_unknown_table`, `analytics_scan_limit_exceeded`, `analytics_query_timeout`, `analytics_statement_not_allowed`. `detail` for the scan-limit case should include the estimate and the limit. The unknown-table `detail` should list available tables and stop referencing the REST path.

## CLI changes

All in `internal/cli/analytics.go` and `internal/client/analytics/client.go` unless noted.

### 1. Help text
- Add `analyticsLongHelp`, `analyticsQueryLongHelp`, `analyticsSchemaLongHelp` consts following the `searchLongHelp` pattern (`internal/cli/search.go:21-26`). Cover: run `schema` first, BigQuery Standard SQL, only SELECT, daily `timestamp` buckets, always filter on `timestamp` for the per-page and referrer tables, limits are reported by `schema`, stdout is JSON, truncation warning on stderr.
- Replace the broken example with several real ones: a `schema` call, a 7-day `SUM(count)` grouped by `user_agent`, a per-path query with a date filter, and a stdin example.
- `Args`: separate messages for zero args ("analytics query requires <SQL>") and extra args ("analytics query accepts a single <SQL> argument"); reject blank SQL like `search.go:46-48`.
- Remove the `--user-agent` flag from `query` and `schema`. Identity resolves from the stored profile with no override.

### 2. Input
- `query -` reads SQL from stdin (`cmd.InOrStdin()`, `io.ReadAll`, trim). Error if empty. No `--file` flag.

### 3. Output formats
- Add `--format json|table|csv` on `query` (default `json`) and `--format json|table` on `schema`. Invalid value is a `UsageError`.
- `table`: `text/tabwriter` with the same params as `auth status` (`internal/cli/auth.go:564-576`), header row from `columns`, NULL rendered as empty.
- `csv`: stdlib `encoding/csv`, header row, NULL as empty.
- JSON output is the raw `QueryResponse` including `meta` when present (add `Meta *QueryMeta` with `json:"meta,omitempty"`).
- Schema `table` view: one block per table (name, description, columns with type and description and values), then a "Limits:" block listing every entry of the map, then "Dialect:". JSON view is the raw object.

### 4. Truncation and metadata
- After a successful query, if `meta.truncated` is true, print to stderr via `printLeadingCommand`-style helper: `warning: result truncated at <row_count> rows (server limit). Add ORDER BY and LIMIT/OFFSET to page, or narrow the query.` Number comes from `meta.row_count`, never a constant.
- Nothing is printed when `meta` is absent (old server).

### 5. Schema decoding
- Client `Schema` returns a new `SchemaResponse{Dialect string; Tables []QueryTable; Limits map[string]Limit}`. Decode into `json.RawMessage`, sniff first non-space byte: `[` means legacy array of tables, `{` means the new object. Both paths produce `SchemaResponse`.
- `QueryTable` and `QueryColumn` gain `Description string` and `QueryColumn` gains `Values []string`, all `omitempty`.

### 6. Error hints
- In `runAnalyticsQuery`, after `Query` fails, inspect the error with `errors.As` for `*problemjson.Problem`. Map `Code` to a stderr hint line appended after the error:
  - `analytics_unknown_table` and `analytics_statement_not_allowed`: "Run `tollbit analytics schema` to list available tables."
  - `analytics_scan_limit_exceeded` and `analytics_query_timeout`: "Run `tollbit analytics schema` to see query limits, then filter on timestamp or select fewer columns."
- Unknown or absent codes keep today's behavior. No string matching on `detail`.

### 7. Client headers and token flow
- `Query` and `Schema` set `User-Agent: version.HTTPUserAgent()` and `X-Tollbit-Client: version.ClientHeader()`, matching `internal/client/tollbit/client.go:355-357`. No `Tollbit-User-Agent` header and no signature change.
- Switch both runners to the `RetryOnOBORequired` branch used by search/pricing/fetch (`internal/cli/search.go:112-122`) so behavior matches the rest of the CLI.

### 8. Not doing
- Hiding `--end-user-proximity` from analytics help. No precedent for hiding flags; it is one line of noise.
- Version bump. Release is a separate PR via `make bump`.
- Removing dead `trim`/`joinArgs` in `common.go`.

## Skill and docs

- `skill/tollbit-cli/SKILL.md`: add an `## Analytics` section between Fetch and Auth: purpose (org traffic analytics), always run `schema` first and read `limits`, dialect, only SELECT, `timestamp` is a daily bucket and the four log tables need a date filter, `type` meaning until the server documents it, JSON shape, `--format`, stdin, truncation warning, error hints. Include three example queries matching the help. Extend the `## For automation` bullets: `--format` on analytics, and "check `meta.truncated`". Update the frontmatter `description` to add "...or query the org's site traffic analytics (bot and referrer logs)". Keep `version: 0.3.2`.
- `README.md`: add an `analytics query` / `analytics schema` row to the command table at lines 74-82 and a short section after Feedback with two examples.

## Server backlog (evidence in `analytics-cli-review.md`)

Contract (needed for CLI items 4, 5, 6):
1. Schema object with `dialect`, `tables[].description`, `columns[].description`, `columns[].values`, and `limits` map as above.
2. Query `meta` with `row_count`, `truncated`, `bytes_scanned`, `duration_ms`.
3. ProblemJSON `code` values listed above; scan-limit `detail` includes numbers; unknown-table `detail` lists tables; multi-statement error should say "multiple statements are not supported".
4. Over-limit queries should never surface as 500 (seen once on a 330-day query).

Data model:
5. `agent_logs_by_page` and `page_logs_by_agent` are identical. Drop one or document the difference.
6. `page_logs_for_referrers` and `referrer_logs` differ only by today's rows. Same ask.
7. `normalized_landing_path` never differs from `landing_path` over 90 days. Drop or fix normalization.
8. `type = ROBOT` means "/robots.txt request", not "is a bot". Rename to `request_kind` or document via `values` descriptions.
9. `status_code` holds only 200/300/400/500. Rename to `status_class` or document.
10. No bot/human/AI classification column. `ip_provider` is null for 99% of rows. Add a `client_class` or similar.
11. `timestamp` is a day bucket typed TIMESTAMP. Consider a `day` DATE column or document.
12. Empty-string `user_agent` vs NULL; `user_agent` is sometimes a family name and sometimes raw. Document normalization.
13. `Edge Health Probe` dominates one host. Consider excluding infra probes or tagging them.
14. `full_referrer` is sometimes an origin and sometimes a full URL. Document.

## Files

- `internal/cli/analytics.go` (help, args, stdin, formats, truncation warning, hints, OBO retry, drop `--user-agent`)
- `internal/cli/analytics_test.go` (new cases below)
- `internal/client/analytics/client.go` and `client_test.go` (headers, `SchemaResponse`, `Meta`, legacy array decode)
- `skill/tollbit-cli/SKILL.md`
- `README.md`

## Verification

- `make test` (runs both `-tags dev` and release via CI; locally `go test ./... && go test -tags dev ./...`).
- New CLI tests: `--format table` and `csv` render header plus NULL as empty; `query -` reads stdin; blank SQL is a usage error; extra args message; truncation warning appears on stderr only when `meta.truncated`; schema decodes both array and object bodies and renders limits generically (a limit name the test invents must print); ProblemJSON `code` maps to the hint on stderr; unknown code prints no hint.
- New client tests: request carries `User-Agent` and `X-Tollbit-Client`; `Meta` decodes; legacy schema array decodes.
- CLI test: `--user-agent` on `analytics query` or `analytics schema` is rejected as an unknown flag (exit 2).
- Skill tests: `TestSkillFrontmatterVersionMatchesCLI` still passes; rendered guide has no `{{`.
- Manual: `make build`, then `./tollbit analytics --help`, `./tollbit analytics query --help`, `./tollbit analytics schema --format table`, `echo 'SELECT 1 AS x' | ./tollbit analytics query -`, and a real 7-day query with `--format table` against the live gateway. The live server still returns the legacy schema array, so limits will print only once the server ships.
