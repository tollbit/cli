package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	analyticsclient "github.com/tollbit/cli/internal/client/analytics"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
)

func NewAnalyticsCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Query TollBit analytics",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("analytics requires a subcommand")
			}
			return UsageError("unknown analytics command %q", args[0])
		},
	}
	cmd.AddCommand(NewAnalyticsQueryCommand(factory))
	return cmd
}

func NewAnalyticsQueryCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "query <SQL>",
		Short:   "Execute an analytics SQL query",
		Example: "  tollbit analytics query 'SELECT * FROM logs LIMIT 10'",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError("analytics query requires <SQL>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyticsQuery(cmd, factory, args[0])
		},
	}
	cmd.Flags().String("user-agent", "", "user agent for request")
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
	identity, err := credentials.ResolveIdentity(cmd.Context(), agenttoken.ResolveIdentityOptions{
		UserAgent: flagChangedStr(cmd, "user-agent"),
	})
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}
	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO())
	if err != nil {
		return RuntimeError(fmt.Errorf("error fetching agent token: %w", err))
	}
	result, err := analyticsClient.Query(cmd.Context(), analyticsclient.QueryRequest{SQL: sql}, token)
	if err != nil {
		return RuntimeError(fmt.Errorf("error querying analytics: %w", err))
	}
	if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
		return RuntimeError(fmt.Errorf("error writing analytics response: %w", err))
	}
	return nil
}
