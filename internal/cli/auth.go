package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli/globalflags"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/tokens/agent"
)

type (
	authLoginOptions struct {
		name             string
		userAgent        string
		useRefreshTokens bool
	}

	authStatusOptions struct {
		asJSON bool
		check  bool
	}

	authSetOptions struct {
		name      string
		userAgent string
	}

	authLogoutOptions struct {
		all   bool
		force bool
	}
)

func NewAuthCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage your agent profile and authorization token",
		Long:  "Manages your agent's profile and authorization token.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("auth requires login, logout, status, or set")
			}
			return UsageError("unknown auth command %q", args[0])
		},
	}
	cmd.AddCommand(
		NewAuthLoginCommand(factory),
		NewAuthLogoutCommand(factory),
		NewAuthStatusCommand(factory),
		NewAuthSetCommand(factory),
	)
	return cmd
}

func NewAuthLoginCommand(factory app.Factory) *cobra.Command {
	var opts authLoginOptions
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize this agent with a Tollbit user and organization",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("auth login does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogin(cmd, factory, opts)
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", "agent name to authorize")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "user agent sent when minting the agent token")
	cmd.Flags().BoolVar(&opts.useRefreshTokens, "use-refresh-tokens", factory.Config.Auth.UseRefreshTokens, "request offline access and store refresh tokens for this agent")
	return cmd
}

func NewAuthLogoutCommand(factory app.Factory) *cobra.Command {
	var opts authLogoutOptions
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the agent authorization token",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("auth logout does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthLogout(cmd, factory, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.all, "all", false, "also clear the persisted agent profile")
	cmd.Flags().BoolVar(&opts.force, "force", false, "clear local credentials even if the server token could not be revoked")
	return cmd
}

func NewAuthStatusCommand(factory app.Factory) *cobra.Command {
	var opts authStatusOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show agent profile and authorization status",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("auth status does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthStatus(cmd, factory, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print status as JSON")
	cmd.Flags().BoolVar(&opts.check, "check", false, "exit 0 if valid, 1 if invalid/expired, 2 if missing (no stdout)")
	return cmd
}

func NewAuthSetCommand(factory app.Factory) *cobra.Command {
	var opts authSetOptions
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update the persisted agent profile",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("auth set does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthSet(cmd, factory, opts)
		},
	}
	cmd.Flags().StringVar(&opts.name, "name", "", "agent name")
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", "", "registered TollBit user agent")
	return cmd
}

func runAuthLogin(cmd *cobra.Command, factory app.Factory, opts authLoginOptions) error {
	overrides, err := globalflags.OverridesFromCommand(cmd)
	if err != nil {
		return RuntimeError(err)
	}
	overrides.AuthUseRefreshTokens = &opts.useRefreshTokens
	app, err := factory.New(overrides)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	ctx := cmd.Context()
	identityOpts := agenttoken.ResolveIdentityOptions{
		Name:      flagChangedStr(cmd, "name"),
		UserAgent: flagChangedStr(cmd, "user-agent"),
	}
	identity, err := credentials.ResolveIdentity(ctx, identityOpts)
	if err != nil {
		return RuntimeError(err)
	}

	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO(), agenttoken.WithRefreshTokens(opts.useRefreshTokens))
	if err != nil {
		return RuntimeError(err)
	}
	if err := credentials.WriteIdentity(ctx, identity); err != nil {
		return RuntimeError(err)
	}

	claims, err := token.Claims()
	if err != nil {
		return RuntimeError(err)
	}

	stderr := cmd.ErrOrStderr()
	msg := fmt.Sprintf("authorized as %s", identity.Name)
	if claims.OBO != nil {
		parts := make([]string, 0, 2)
		if claims.OBO.User != "" {
			parts = append(parts, "user "+claims.OBO.User)
		}
		if claims.OBO.Org != "" {
			parts = append(parts, "org "+claims.OBO.Org)
		}
		if len(parts) > 0 {
			msg += " (on behalf of " + strings.Join(parts, " / ") + ")"
		}
	}
	fmt.Fprintln(stderr, msg)
	return nil
}

func runAuthLogout(cmd *cobra.Command, factory app.Factory, opts authLogoutOptions) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	ctx := cmd.Context()

	var clearErr error
	successMsg := "Cleared agent token."
	if opts.all {
		clearErr = credentials.ClearIdentity(ctx, opts.force)
		successMsg = "Cleared agent profile and token."
	} else {
		clearErr = credentials.ClearAgentTokens(ctx, opts.force)
	}

	switch {
	case clearErr == nil:
		fmt.Fprintln(cmd.OutOrStdout(), successMsg)
		return nil
	case errors.Is(clearErr, agenttoken.ErrRevokeFailed) && opts.force:
		fmt.Fprintln(cmd.OutOrStdout(), successMsg)
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: could not revoke the token on the server. It will be revoked the next time you log in, or expires within 30 days.")
		return nil
	case errors.Is(clearErr, agenttoken.ErrRevokeFailed):
		return RuntimeError(errors.New(
			"could not reach the server to revoke your token; you are still logged in. " +
				"Check your connection and run `tollbit auth logout` again. " +
				"To clear local credentials without revoking, use --force (the token is revoked at your next login or expires within 30 days)."))
	default:
		return RuntimeError(clearErr)
	}
}

func runAuthStatus(cmd *cobra.Command, factory app.Factory, opts authStatusOptions) error {
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

	if opts.check {
		if !tokenExists {
			return ExitError{Code: 2, Err: errors.New("agent token missing")}
		}
		if tokenErr != nil {
			return ExitError{Code: 1, Err: tokenErr}
		}
		return nil
	}

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
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stdout, "Agent:      %s\n", identity.Name)
	if identity.UserAgent != "" {
		fmt.Fprintf(stdout, "User agent: %s\n", identity.UserAgent)
	} else {
		fmt.Fprintf(stdout, "User agent:\n")
	}
	printAuthTokenStatus(stdout, token, tokenExists, tokenErr)
	if tokenExists && tokenErr == nil {
		if claims, claimsErr := token.Claims(); claimsErr == nil && claims.Subject != "" && claims.Subject != identity.Name {
			fmt.Fprintf(stderr, "token subject %q does not match profile name %q — run 'tollbit auth login'\n", claims.Subject, identity.Name)
		}
	}
	return nil
}

func runAuthSet(cmd *cobra.Command, factory app.Factory, opts authSetOptions) error {
	if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("user-agent") {
		return UsageError("auth set requires --name and/or --user-agent")
	}
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	identity, err := credentials.ResolveIdentity(cmd.Context(), agenttoken.ResolveIdentityOptions{
		Name:      flagChangedStr(cmd, "name"),
		UserAgent: flagChangedStr(cmd, "user-agent"),
	})
	if err != nil {
		return RuntimeError(err)
	}
	if err := credentials.SaveIdentity(cmd.Context(), identity); err != nil {
		return RuntimeError(err)
	}
	stdout := cmd.OutOrStdout()
	stderr := cmd.ErrOrStderr()
	fmt.Fprintf(stdout, "updated agent profile %s\n", identity.Name)
	fmt.Fprintln(stderr, "cleared token — profile updated; run 'tollbit auth login'")
	return nil
}

func printAuthTokenStatus(w interface{ Write([]byte) (int, error) }, token agent.Token, exists bool, validationErr error) {
	if !exists {
		fmt.Fprintln(w, "Token:      none")
		return
	}
	if validationErr != nil {
		expires := tokenExpiryLabel(token)
		if expires != "" {
			fmt.Fprintf(w, "Token:      expired (%s)\n", expires)
			return
		}
		fmt.Fprintf(w, "Token:      invalid (%v)\n", validationErr)
		return
	}
	claims, err := token.Claims()
	if err != nil {
		fmt.Fprintf(w, "Token:      invalid (%v)\n", err)
		return
	}
	expires := "unknown"
	if claims.ExpiresAt != nil {
		expires = claims.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "Token:      valid (expires %s)\n", expires)
	if claims.OBO == nil {
		return
	}
	parts := make([]string, 0, 2)
	if claims.OBO.User != "" {
		parts = append(parts, "user "+claims.OBO.User)
	}
	if claims.OBO.Org != "" {
		parts = append(parts, "org "+claims.OBO.Org)
	}
	if len(parts) == 0 {
		return
	}
	source := strings.TrimSpace(claims.OBO.Source)
	suffix := ""
	if source != "" {
		suffix = " (" + source + ")"
	}
	fmt.Fprintf(w, "On behalf:  %s%s\n", strings.Join(parts, " / "), suffix)
}

func tokenExpiryLabel(token agent.Token) string {
	claims, err := token.Claims()
	if err != nil || claims.ExpiresAt == nil {
		return ""
	}
	return claims.ExpiresAt.Time.UTC().Format(time.RFC3339)
}
