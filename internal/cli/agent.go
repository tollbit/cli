package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/credentials/agenttoken"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
)

type (
	agentLoginOptions struct {
		agentName string
		userAgent string
	}

	agentStatusOptions struct {
		asJSON bool
	}
)

func NewAgentCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Authorize this agent with Tollbit",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("agent requires login, status, or logout")
			}
			return UsageError("unknown agent command %q", args[0])
		},
	}
	cmd.AddCommand(
		NewAgentLoginCommand(factory),
		NewAgentStatusCommand(factory),
		NewAgentLogoutCommand(factory),
	)
	return cmd
}

func NewAgentLoginCommand(factory app.Factory) *cobra.Command {
	var opts agentLoginOptions
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize this agent with a Tollbit user and organization",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("agent login does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentLogin(cmd, factory, opts)
		},
	}
	cmd.Flags().StringVar(&opts.agentName, "agent-name", "", "agent name to authorize")
	cmd.Flags().StringVar(&opts.userAgent, "agent-user-agent", "", "user agent sent when minting the agent identity token")
	return cmd
}

func NewAgentStatusCommand(factory app.Factory) *cobra.Command {
	var opts agentStatusOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show agent authorization status",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("agent status does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentStatus(cmd, factory, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print status as JSON")
	return cmd
}

func NewAgentLogoutCommand(factory app.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear agent token",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("agent logout does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentLogout(cmd, factory)
		},
	}
}

func runAgentLogin(cmd *cobra.Command, factory app.Factory, opts agentLoginOptions) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	ctx := cmd.Context()
	identityOpts := agenttoken.ResolveIdentityOptions{
		Name:      flagChangedStr(cmd, "agent-name"),
		UserAgent: flagChangedStr(cmd, "agent-user-agent"),
	}
	identity, err := credentials.ResolveIdentity(ctx, identityOpts)
	if err != nil {
		return RuntimeError(err)
	}
	stdout := cmd.OutOrStdout()
	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO())
	if err != nil {
		return RuntimeError(err)
	}
	claims, err := token.Claims()
	if err != nil {
		return RuntimeError(err)
	}

	fmt.Fprintln(stdout, "Agent authorized.")
	fmt.Fprintf(stdout, "Agent: %s\n", identity.Name)
	if claims.ExpiresAt != nil {
		fmt.Fprintf(stdout, "Agent token expires: %s\n", claims.ExpiresAt.Time.UTC().Format(time.RFC3339))
	}
	if claims.OBO != nil {
		if claims.OBO.User != "" {
			fmt.Fprintf(stdout, "User: %s\n", claims.OBO.User)
		}
		if claims.OBO.Org != "" {
			fmt.Fprintf(stdout, "Organization: %s\n", claims.OBO.Org)
		}
		if claims.OBO.Source != "" {
			fmt.Fprintf(stdout, "Source: %s\n", claims.OBO.Source)
		}
	}
	return nil
}

func runAgentStatus(cmd *cobra.Command, factory app.Factory, opts agentStatusOptions) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	ctx := cmd.Context()
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	identity, err := credentials.GetIdentity(ctx)
	if err != nil {
		return RuntimeError(err)
	}
	token, tokenExists, tokenErr := credentials.CurrentAgentToken(ctx)

	status := map[string]any{
		"identity": map[string]string{
			"name":       identity.Name,
			"user_agent": identity.UserAgent,
		},
		"token": agenttoken.Status(token, tokenExists, tokenErr),
	}
	if opts.asJSON {
		return RuntimeError(writeJSON(cmd.OutOrStdout(), status))
	}

	stdout := cmd.OutOrStdout()
	fmt.Fprintf(stdout, "Agent: %s\n", identity.Name)
	if identity.UserAgent != "" {
		fmt.Fprintf(stdout, "User agent: %s\n", identity.UserAgent)
	}
	printAgentTokenStatus(stdout, token, tokenExists, tokenErr)
	return nil
}

func runAgentLogout(cmd *cobra.Command, factory app.Factory) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	if err := credentials.ClearAgentTokens(cmd.Context()); err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Cleared agent token.")
	return nil
}

func printTokenStatus(w interface{ Write([]byte) (int, error) }, label string, token agent.Token, exists bool, validationErr error) {
	if !exists {
		fmt.Fprintf(w, "%s: missing\n", label)
		return
	}
	if validationErr != nil {
		fmt.Fprintf(w, "%s: invalid (%v)\n", label, validationErr)
		return
	}
	claims, err := token.Claims()
	if err != nil {
		fmt.Fprintf(w, "%s: invalid (%v)\n", label, err)
		return
	}
	expires := "unknown"
	if claims.ExpiresAt != nil {
		expires = claims.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "%s: valid subject=%s expires=%s\n", label, claims.Subject, expires)
}

func printAgentTokenStatus(w interface{ Write([]byte) (int, error) }, token agent.Token, exists bool, validationErr error) {
	printTokenStatus(w, "Agent token", token, exists, validationErr)
	if !exists || validationErr != nil {
		return
	}
	claims, err := token.Claims()
	if err != nil || claims.OBO == nil {
		fmt.Fprintln(w, "Authorization: missing")
		return
	}
	fmt.Fprintln(w, "Authorization: present")
	if claims.OBO.User != "" {
		fmt.Fprintf(w, "User: %s\n", claims.OBO.User)
	}
	if claims.OBO.Org != "" {
		fmt.Fprintf(w, "Organization: %s\n", claims.OBO.Org)
	}
	if claims.OBO.Source != "" {
		fmt.Fprintf(w, "Source: %s\n", claims.OBO.Source)
	}
}
