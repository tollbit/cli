package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/client/tollbit"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/tokens/agent"
)

const feedbackLongHelp = `Submit feedback about the TollBit CLI or agent experience.

Requires an authenticated agent with on-behalf-of (OBO) consent. Feedback is
accepted asynchronously and delivered to Tollbit (Slack + spreadsheet).`

type feedbackOptions struct {
	rating    int
	category  string
	metadata  []string
	userAgent string
	asJSON    bool
}

func NewFeedbackCommand(factory app.Factory) *cobra.Command {
	var opts feedbackOptions

	cmd := &cobra.Command{
		Use:   `feedback "message"`,
		Short: "Submit feedback to Tollbit",
		Long:  feedbackLongHelp,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError(`feedback requires exactly one message argument`)
			}
			if strings.TrimSpace(args[0]) == "" {
				return UsageError("feedback message must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedback(cmd, factory, opts, strings.Trim(args[0], `"'`))
		},
	}

	cmd.Flags().IntVar(&opts.rating, "rating", 0, "optional rating from 1 (worst) to 5 (best)")
	cmd.Flags().StringVar(&opts.category, "category", "", "optional category label (e.g. search, auth, content)")
	cmd.Flags().StringArrayVar(&opts.metadata, "metadata", nil, "optional key=value context (repeatable)")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "user agent for request")
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "emit raw JSON response")

	return cmd
}

func runFeedback(cmd *cobra.Command, factory app.Factory, opts feedbackOptions, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return UsageError("feedback message must not be empty")
	}

	req := tollbit.SubmitFeedbackRequest{
		Message:  message,
		Category: strings.TrimSpace(opts.category),
	}
	if cmd.Flags().Changed("rating") {
		if opts.rating < 1 || opts.rating > 5 {
			return UsageError("feedback --rating must be between 1 and 5")
		}
		rating := opts.rating
		req.Rating = &rating
	}
	metadata, err := parseMetadataFlags(opts.metadata)
	if err != nil {
		return err
	}
	req.Metadata = metadata

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

	var resp tollbit.SubmitFeedbackResponse
	if app.Config().Auth.RetryOnOBORequired {
		resp, err = agenttoken.WithOBORetry(cmd, credentials, identity, func(token agent.Token) (tollbit.SubmitFeedbackResponse, error) {
			return tollbitClient.SubmitFeedback(cmd.Context(), req, token)
		})
	} else {
		token, tokenErr := credentials.GetAgentToken(cmd, identity)
		if tokenErr != nil {
			return RuntimeError(fmt.Errorf("error fetching agent token: %w", tokenErr))
		}
		resp, err = tollbitClient.SubmitFeedback(cmd.Context(), req, token)
	}
	if err != nil {
		return RuntimeError(fmt.Errorf("error submitting feedback: %w", err))
	}

	if opts.asJSON {
		return RuntimeError(writeJSON(cmd.OutOrStdout(), resp))
	}
	if resp.Accepted {
		fmt.Fprintln(cmd.OutOrStdout(), "Feedback accepted.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Feedback was not accepted.")
	}
	return nil
}

func parseMetadataFlags(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		key, value, ok := strings.Cut(raw, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, UsageError("feedback --metadata must be key=value, got %q", raw)
		}
		out[key] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
