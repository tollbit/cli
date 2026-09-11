# `tollbit analytics` usability review

Date: 2026-09-11. CLI version 0.3.2. Method: help text, `tollbit guide`, and live queries only. No source code consulted.

## Summary

The analytics commands work: `schema` lists five tables, `query` accepts BigQuery-style SQL (CTEs, joins, `COUNTIF`, `FORMAT_TIMESTAMP`, string date literals all work), only SELECT is allowed, and bad-column errors are helpful. But a first-time user hits several walls that the help does not warn about.

Top issues, ranked:

1. **Silent 10,000-row cap.** A full-table select returns exactly 10,000 rows, exit 0, with no truncation marker in the JSON and no note in `--help`. `LIMIT 20000` also returns 10,000.
2. **Undocumented scan limit.** `SELECT * FROM agent_logs_by_page LIMIT 2` fails with a 422 "would scan more than the allowed limit". LIMIT does not help, no number is given, and the safe date window depends on which columns you select. One boundary query surfaced as a 500 instead.
3. **The only help example is broken.** `SELECT * FROM logs LIMIT 10` fails because no `logs` table exists, and the error points at a REST endpoint instead of `tollbit analytics schema`.
4. **Duplicate tables.** `agent_logs_by_page` and `page_logs_by_agent` are identical in content. The two referrer tables differ only by what looks like today's refresh lag.
5. **Misleading column names.** `type = ROBOT` means "requested /robots.txt", not "is a bot", so Chrome and Safari show up as ROBOT. `status_code` holds only 200/300/400/500 classes. `normalized_landing_path` never differs from `landing_path`.
6. **No bot / AI classification column.** Answering "which AI crawlers hit my site" requires knowing agent names in advance. `ip_provider` is null for 99% of traffic.
7. **No table descriptions, no SQL dialect stated, and the agent guide never mentions analytics.**
8. **Output is JSON only** with positional rows and no metadata, while sibling commands default to human output with a `--json` opt-in.

Questions asked and answered during the pass:

- Top user agents on pioneervalleygazette.com, 30 days: "Edge Health Probe" at 62k hits dwarfs everything else.
- Daily ChatGPT-User / GPTBot / OAI-SearchBot trend, 14 days: works, but only because the agent names were already known.
- Paths GPTBot fetches most: `/` and `/careers` on tollbit.com.
- Referrers to tollbit.com blog pages: mostly self-referrals to WordPress probe paths, then google.com.
- Error rate by host, 30 days: tollbit.com 37% 4xx/5xx, thedailydispatching.com 51%.

## Detailed findings

## A. Documentation / discoverability

1. **The only example in `query --help` fails.** `SELECT * FROM logs LIMIT 10` returns `400 Unknown table`. There is no `logs` table.
2. **Error for unknown table points at a REST endpoint, not the CLI.** It says "Use GET /analytics/agent/v1/query/schema" instead of "run `tollbit analytics schema`".
3. **SQL dialect is never stated.** Errors look like BigQuery (`Unexpected keyword ROWS at [1:72]`, `TIMESTAMP_SUB`, `COUNTIF`, `FORMAT_TIMESTAMP` all work). A user has to guess. `rows` is a reserved word, which bit me on the first aggregate query.
4. **`tollbit guide` has zero mentions of analytics.** The bundled agent skill documents search/pricing/fetch/auth only. An agent following the guide will not know analytics exists.
5. **`schema` output has no table or column descriptions.** Just names and types. None of the questions below can be answered from the schema alone.
6. **No mention of auth requirement, org scoping, or which hosts you will see.** It just worked because I was already logged in; unclear what a fresh user sees.
7. **`--user-agent` on `query` and `schema` has no visible effect** and no explanation of why an analytics query would need one. It accepts any string silently.
8. **No exit-code or error-format documentation.** Errors go to stderr as `error querying analytics: <status>: <message>`, exit 1. Fine, but undocumented.

## B. Result limits (the big ones)

9. **Silent 10,000-row cap.** `SELECT * FROM user_agent_aggregate` (122,552 rows) returns exactly 10,000 rows, exit 0, no warning, no `truncated` flag in the JSON, no note in `--help`. `LIMIT 20000` also returns 10,000. A user summing rows client-side will get wrong answers without knowing.
10. **Rows come back in nondeterministic order without ORDER BY**, so the 10k you get is arbitrary. `LIMIT 1` twice gave rows from 2025-09-08 and 2026-06-02.
11. **Undocumented scan limit.** `SELECT * FROM agent_logs_by_page LIMIT 2` fails with `422 Query would scan more than the allowed limit`. LIMIT does not help. The message says "narrow the date range or the columns selected" but gives no number and no hint of what range is safe. Empirically: `SELECT *` over the full history fails; `SUM(count)` over 365 days works; `COUNT(*), SUM(count)` over 365 days fails; 300 days works for both. So the limit is bytes-scanned and depends on columns, which is invisible to the user.
12. **Same over-limit condition sometimes surfaces as a 500 Internal Server Error** ("The query could not be completed") instead of the 422. Seen once on a 330-day query; three retries then succeeded. Transient, but a 500 reads like an outage, not "your query is too big".
13. **`user_agent_aggregate` can be scanned in full but the other four tables cannot.** Nothing tells you which tables are "small" and which need a date filter.

## C. Table and column semantics

14. **`agent_logs_by_page` and `page_logs_by_agent` are the same table.** Identical columns (only column order differs), identical row count, totals, distinct user agents, and distinct paths over the same window. Why two names?
15. **`page_logs_for_referrers` and `referrer_logs` are almost the same table.** Same columns, different order. Over the same 7-day window one has 8,682 rows and the other 8,691. The extra rows in `referrer_logs` are all from today, so it looks like a refresh lag rather than a semantic difference. Undocumented either way.
16. **`normalized_landing_path` is always equal to `landing_path`.** Zero differing rows over 90 days (72,439 rows). Either the column is redundant or normalization is not running.
17. **`type` values `REQUEST` / `ROBOT` / `SITEMAP` / `WELL-KNOWN` are not explained.** `ROBOT` looks like "is a bot" but it is actually "requests for /robots.txt" (7-day totals: 2,507 ROBOT rows vs 2,506 hits to `/robots.txt`). Chrome, Safari and Firefox all appear with `type = ROBOT`. This is a naming trap for exactly the "how much bot traffic do I get" question this product is about.
18. **There is no bot / human / AI classification column at all.** To answer "which AI crawlers hit my site" you must know the user-agent names yourself. `ip_provider` helps a little but is null for 99% of traffic and is only populated for a handful of providers.
19. **`ip_provider` semantics unclear.** Is it verified-IP-range ownership? It is populated on `ChatGPT-User`, `Googlebot`, etc, but also on `Chrome` (946 hits attributed to `google`), which is confusing without an explanation.
20. **`status_code` is really a status class.** Values are only 200, 300, 400, 500. The column name and INT64 type suggest real codes (301, 404, 429...). A user filtering `status_code = 404` gets nothing with no error.
21. **`user_agent` is a normalized family name, not a raw UA string** ("Chrome", "Googlebot", "Edge Health Probe"), except sometimes it looks raw ("Mozilla/5.0", "Win64"). Empty string `""` is also a value (2.2% of 30-day traffic) and is distinct from NULL.
22. **`timestamp` is a daily bucket at 00:00Z but typed TIMESTAMP.** Every row has hour 0. Today's bucket already exists at 12:00 local, so it is a partial day. A DATE column, or a note, would prevent misreading it as event time. Related footgun: `TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 7 DAY)` silently drops the oldest day bucket compared with `>= '2026-09-04'`.
23. **`count` as a column name collides with `COUNT()`.** It works, but `SUM(count)` vs `COUNT(*)` is easy to mix up and there is no description saying `count` is the pre-aggregated request count.
24. **`full_referrer` is sometimes a bare origin and sometimes a full URL** ("https://www.google.com" vs "https://t.co/xVEzhCBk7X"). Unclear whether paths are stripped for privacy on some domains.

## D. CLI ergonomics

25. **No output format option.** Always JSON. Other commands in the same CLI (`search`, `content`) have `--json` as opt-in with a human default; `analytics` has no human table view and no `--csv`. `--format`, `--json`, `--csv`, `--limit` are all "unknown flag".
26. **JSON shape is `{"columns":[{name,type}],"rows":[[...]]}`.** Positional rows are compact but painful with `jq`; an array of objects (or a flag for it) would be friendlier. No `row_count`, `truncated`, or `bytes_scanned` metadata.
27. **No way to read SQL from stdin or a file.** `query -` sends the literal `-` as SQL. Long multi-line queries must be shell-quoted.
28. **Only one positional arg accepted; a second arg gives the generic "analytics query requires <SQL>"** rather than "too many arguments".
29. **`SHOW TABLES` / `DESCRIBE` are rejected** ("Statement not supported: ShowStatement"). Reasonable, but the error could say "use `tollbit analytics schema`".
30. **Multi-statement queries rejected with a misleading message.** `SELECT 1; SELECT 2` says "Only SELECT statements are supported" even though both are SELECTs.
31. **The `--end-user-proximity` global flag shows on every analytics help page** and is irrelevant to analytics. It adds noise for a command that never triggers browser consent.

## E. Data observations (not bugs, but surprising as a first-time user)

- Four hosts visible for this org: tollbit.com, pioneervalleygazette.com, tollbit.news, thedailydispatching.com. History starts 2025-06-01.
- The top "user agent" on pioneervalleygazette.com over 30 days is `Edge Health Probe` at 62k hits, dwarfing everything else. Probably infra noise that should be filterable or excluded.
- 37% of tollbit.com traffic over 30 days is 4xx/5xx; thedailydispatching.com is 51%. Lots of scanner paths (`/wp-admin`, `///admin.php`, `/www/phpinfo.php`).
- `referrer_logs` "landing paths" include things like `/blog/wp-json/batch/v1` self-referred from tollbit.com, which look like bot probes, not human referrals.
