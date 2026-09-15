package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	analyticsclient "github.com/tollbit/cli/internal/client/analytics"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
)

const analyticsLongHelp = `Query TollBit analytics for your organization's sites.

Start with "analytics schema" to see the tables and columns available to you,
then run SQL with "analytics query". Both commands print JSON to stdout and
errors to stderr, and require an authorized agent token (auth runs
automatically when needed).`

const analyticsQueryLongHelp = `Execute a read-only SQL query against TollBit analytics.

Queries use standard SQL (SQL:2011) as reported by "analytics schema" in its
"dialect" field. Only a single SELECT statement is accepted;
SHOW, DESCRIBE, and data-modifying statements are rejected by the server.
Use "analytics schema" to list the tables and columns you can query.

Rows are daily aggregates: the timestamp column is a day bucket at 00:00 UTC
and the count column is the number of requests in that bucket. Filter on
timestamp (for example, timestamp >= '2026-09-01') to keep queries within the
server's scan limit; the per-page and referrer tables are large enough that
unfiltered queries are rejected even with LIMIT. The server also caps the
number of rows returned, so add ORDER BY with LIMIT and OFFSET when paging
through large results.

Output is a JSON object with "columns" (name and type), "rows" (arrays in
column order, null for missing values), and "meta" (row_count, truncated,
bytes_scanned, duration_ms). When meta.truncated is true the result was cut
at the server's row limit and a warning is printed to stderr.`

const analyticsSchemaLongHelp = `List the analytics tables and columns available to your organization.

Output is a JSON object with "dialect", "tables" and "limits". Each table has
its name, a description, its columns (name, type, description and, where the
column has a fixed set, values), and "clustering": the columns the table is
ordered by, most significant first. Filtering and grouping in that order,
after a timestamp filter, scans the least data. "limits" maps each server
limit to its value, unit and description. Run this before "analytics query".`

const analyticsQueryExample = `  # Discover tables and columns first
  tollbit analytics schema

  # Requests per user agent over the last 7 days
  tollbit analytics query "SELECT user_agent, SUM(count) AS requests FROM user_agent_aggregate WHERE timestamp >= '2026-09-04' GROUP BY user_agent ORDER BY requests DESC LIMIT 20"

  # Most requested paths on one host, with a date filter
  tollbit analytics query "SELECT path, SUM(count) AS requests FROM agent_logs_by_page WHERE host = 'example.com' AND timestamp >= '2026-09-04' GROUP BY path ORDER BY requests DESC LIMIT 20"

  # Daily trend for one crawler
  tollbit analytics query "SELECT timestamp, SUM(count) AS requests FROM user_agent_aggregate WHERE user_agent = 'GPTBot' AND timestamp >= '2026-08-12' GROUP BY timestamp ORDER BY timestamp"`

func NewAnalyticsCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Query TollBit analytics",
		Long:  analyticsLongHelp,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("analytics requires a subcommand")
			}
			return UsageError("unknown analytics command %q", args[0])
		},
	}
	cmd.AddCommand(NewAnalyticsQueryCommand(factory))
	cmd.AddCommand(NewAnalyticsSchemaCommand(factory))
	return cmd
}

func NewAnalyticsQueryCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query <SQL>",
		Short:   "Execute an analytics SQL query",
		Long:    analyticsQueryLongHelp,
		Example: analyticsQueryExample,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("analytics query requires <SQL>")
			}
			if len(args) > 1 {
				return UsageError("analytics query accepts a single <SQL> argument; quote the whole statement")
			}
			if strings.TrimSpace(args[0]) == "" {
				return UsageError("analytics query SQL must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyticsQuery(cmd, factory, args[0])
		},
	}
	return cmd
}

func runAnalyticsQuery(cmd *cobra.Command, factory app.Factory, sql string) error {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := application.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	analyticsClient, err := application.Analytics()
	if err != nil {
		return RuntimeError(err)
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), agenttoken.ResolveIdentityOptions{})
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}
	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO())
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching agent token: %w", err))
	}
	result, err := analyticsClient.Query(cmd.Context(), analyticsclient.QueryRequest{SQL: sql}, token)
	if err != nil {
		if hint := analyticsErrorHint(err); hint != "" {
			printLeadingCommand(cmd.ErrOrStderr(), hint)
		}
		return RuntimeError(fmt.Errorf("error querying analytics: %w", err))
	}
	if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
		return RuntimeError(fmt.Errorf("error writing analytics response: %w", err))
	}
	if result.Meta != nil && result.Meta.Truncated {
		printLeadingCommand(cmd.ErrOrStderr(), fmt.Sprintf("warning: result truncated at %d rows (server limit). Add ORDER BY with LIMIT and OFFSET to page, or narrow the query.", result.Meta.RowCount))
	}
	return nil
}

// analyticsErrorHint maps a server error code to a next step. Unknown or
// absent codes give no hint.
func analyticsErrorHint(err error) string {
	var problem problemjson.Problem
	if !errors.As(err, &problem) || problem.Code == nil {
		return ""
	}
	switch string(*problem.Code) {
	case "analytics_unknown_table", "analytics_statement_not_allowed":
		return "Run \"tollbit analytics schema\" to list the available tables."
	case "analytics_scan_limit_exceeded", "analytics_query_timeout":
		return "Run \"tollbit analytics schema\" to see the query limits, then filter on timestamp or select fewer columns."
	}
	return ""
}

func NewAnalyticsSchemaCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "List available analytics tables and columns",
		Long:    analyticsSchemaLongHelp,
		Example: "  tollbit analytics schema",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("analytics schema accepts no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyticsSchema(cmd, factory)
		},
	}
	return cmd
}

func runAnalyticsSchema(cmd *cobra.Command, factory app.Factory) error {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := application.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	analyticsClient, err := application.Analytics()
	if err != nil {
		return RuntimeError(err)
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), agenttoken.ResolveIdentityOptions{})
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}
	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO())
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching agent token: %w", err))
	}
	result, err := analyticsClient.Schema(cmd.Context(), token)
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching analytics schema: %w", err))
	}
	if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
		return RuntimeError(fmt.Errorf("error writing analytics schema: %w", err))
	}
	return nil
}
