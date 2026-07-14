package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/client/tollbit"
	"github.com/tollbit/tollbit-cli/internal/credentials/agenttoken"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

const (
	searchMaxSize            = 20
	searchMaxProperties      = 20
	searchDefaultSize        = 10
)

const searchLongHelp = `Search content on the TollBit network.

Results show access type: Programmatic (licensable via the CLI now) or
Enterprise (reach out to Tollbit for access). Use --programmatic-only to
limit results to Programmatic content. Without it, search spans the full
catalog of discoverable content.`

type searchOptions struct {
	size             int
	nextToken        string
	properties       string
	programmaticOnly bool
	userAgent        string
	asJSON           bool
}

func NewSearchCommand(factory app.Factory) *cobra.Command {
	var opts searchOptions

	cmd := &cobra.Command{
		Use:   `search "query"`,
		Short: "Search content on the TollBit network",
		Long:  searchLongHelp,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError(`search requires exactly one query argument`)
			}
			if strings.TrimSpace(args[0]) == "" {
				return UsageError("search query must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd, factory, opts, strings.Trim(args[0], `"'`))
		},
	}

	cmd.Flags().IntVar(&opts.size, "size", searchDefaultSize, "number of results to return (max 20)")
	cmd.Flags().StringVar(&opts.nextToken, "next-token", "", "pagination token from a previous search")
	cmd.Flags().StringVar(&opts.properties, "properties", "", "comma-separated domains to boost (max 20)")
	cmd.Flags().BoolVar(&opts.programmaticOnly, "programmatic-only", false, "limit results to Programmatic content")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "user agent for request")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "emit raw JSON response")

	return cmd
}

func runSearch(cmd *cobra.Command, factory app.Factory, opts searchOptions, query string) error {
	if opts.size < 1 || opts.size > searchMaxSize {
		return UsageError("search --size must be between 1 and %d", searchMaxSize)
	}
	properties := strings.TrimSpace(opts.properties)
	if properties != "" {
		domains := splitCommaSeparated(properties)
		if len(domains) > searchMaxProperties {
			return UsageError("search --properties accepts at most %d domains", searchMaxProperties)
		}
		properties = strings.Join(domains, ",")
	}

	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	tollbitClient, err := app.Tollbit()
	if err != nil {
		return RuntimeError(err)
	}

	identityOpts := agenttoken.ResolveIdentityOptions{
		UserAgent: flagChangedStr(cmd, "user-agent"),
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), identityOpts)
	if err != nil {
		return RuntimeError(fmt.Errorf("error resolving identity: %w", err))
	}

	params := tollbit.SearchParams{
		Query:               query,
		Size:                opts.size,
		NextToken:           opts.nextToken,
		Properties:          properties,
		ProgrammaticOnly:    opts.programmaticOnly,
		ProgrammaticOnlySet: cmd.Flags().Changed("programmatic-only"),
	}

	var resp tollbit.PagedSearchResultResponse
	if app.Config().Auth.RetryOnOBORequired {
		resp, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) (tollbit.PagedSearchResultResponse, error) {
			return tollbitClient.Search(cmd.Context(), params, token)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		resp, err = tollbitClient.Search(cmd.Context(), params, token)
	}
	if err != nil {
		return RuntimeError(fmt.Errorf("error searching: %w", err))
	}

	if opts.asJSON {
		return RuntimeError(writeJSON(cmd.OutOrStdout(), resp))
	}
	printSearchResults(cmd.OutOrStdout(), resp)
	return nil
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func printSearchResults(w io.Writer, resp tollbit.PagedSearchResultResponse) {
	if len(resp.Items) == 0 {
		fmt.Fprintln(w, "No results.")
	} else {
		for i, item := range resp.Items {
			fmt.Fprintf(w, "%d. %s\n", i+1, item.Title)
			fmt.Fprintf(w, "   %s\n", item.URL)
			fmt.Fprintf(w, "   %s (%s) · %s\n", item.Publisher.Name, item.Publisher.Domain, item.PublishedDate)
			fmt.Fprintf(w, "   %s\n", formatAvailabilityLabels(item.Availability))
		}
	}
	if next := strings.TrimSpace(resp.NextToken); next != "" {
		fmt.Fprintf(w, "\nMore results available. Pass --next-token %q to continue.\n", next)
	}
	if len(resp.Items) > 0 {
		printLeadingCommand(w, "To get pricing: tollbit content pricing <url>[,<url>...]")
	}
}

func formatAvailabilityLabels(a tollbit.Availability) string {
	labels := make([]string, 0, 2)
	if a.Discoverable {
		labels = append(labels, "In TollBit network")
	}
	if a.ReadyToLicense {
		labels = append(labels, "Programmatic")
	} else {
		labels = append(labels, "Enterprise")
	}
	return strings.Join(labels, " · ")
}
