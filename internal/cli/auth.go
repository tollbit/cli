package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli/globalflags"
	"github.com/tollbit/cli/internal/cliruntime"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/credentials/agenttoken"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
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
				return UsageError("auth requires login, complete, logout, status, or set")
			}
			return UsageError("unknown auth command %q", args[0])
		},
	}
	cmd.AddCommand(
		NewAuthLoginCommand(factory),
		NewAuthCompleteCommand(factory),
		NewAuthLogoutCommand(factory),
		NewAuthStatusCommand(factory),
		NewAuthSetCommand(factory),
	)
	return cmd
}

func NewAuthCompleteCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete",
		Short: "Complete a pending detached agent authorization",
		Long:  authCompleteLongHelp(factory),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 3 {
				return UsageError("auth complete accepts at most 3 icon name arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthComplete(cmd, factory, args)
		},
	}
	return cmd
}

func NewAuthLoginCommand(factory app.Factory) *cobra.Command {
	var opts authLoginOptions
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize this agent with a Tollbit user and organization",
		Long:  authLoginLongHelp(factory),
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

func authCompleteLongHelp(factory app.Factory) string {
	preamble := strings.TrimSpace(`Complete a pending detached agent authorization.

Use this after auth login starts a detached authorization flow. The command
checks once and exits. If authorization is still pending, the pending auth record
is kept so the command can be run again after the end user completes the required
browser steps. If the challenge is denied, expired, or invalid, the pending auth
record is cleared. If authorization is still pending, this exits with code 3.`)
	local, localOK := configuredStrategyGuidance(factory, factory.Config.Auth.Consent.Strategy.Local)
	remote, remoteOK := configuredStrategyGuidance(factory, factory.Config.Auth.Consent.Strategy.Remote)
	if !localOK || !remoteOK {
		return preamble + "\n\nRun `tollbit guide` for flow instructions."
	}
	return strings.TrimSpace(fmt.Sprintf("%s\n\nConfigured local completion flow (%s):\n%s\n\nConfigured remote completion flow (%s):\n%s", preamble, local.FlowLabel, local.CompleteInstructions, remote.FlowLabel, remote.CompleteInstructions))
}

func authLoginLongHelp(factory app.Factory) string {
	preamble := strings.TrimSpace(`Authorize this agent with a Tollbit user and organization.

End-user proximity describes where the agent environment running this CLI is
located relative to the end user's browser. The CLI uses that context to present
either the local or the detached (remote) flow. When a detached flow starts, this
command exits with code 3 and keeps the pending authorization.`)
	local, localOK := configuredStrategyGuidance(factory, factory.Config.Auth.Consent.Strategy.Local)
	remote, remoteOK := configuredStrategyGuidance(factory, factory.Config.Auth.Consent.Strategy.Remote)
	if !localOK || !remoteOK {
		return preamble + "\n\nRun `tollbit guide` for flow instructions."
	}
	return strings.TrimSpace(fmt.Sprintf("%s\n\nConfigured local login flow (%s):\n%s\n\nConfigured remote login flow (%s):\n%s", preamble, local.FlowLabel, local.LoginInstructions, remote.FlowLabel, remote.LoginInstructions))
}

func configuredStrategyGuidance(factory app.Factory, strategyName string) (agentauth.ConsentGuidance, bool) {
	application, err := factory.New(configuration.OverrideOptions{})
	if err != nil {
		return agentauth.ConsentGuidance{}, false
	}
	strategy, err := application.ConsentStrategy(agentauth.ConsentMethod(strategyName))
	if err != nil {
		return agentauth.ConsentGuidance{}, false
	}
	return strategy.Guidance(), true
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
	if err := printAuthLoginRuntimeContext(cmd, app); err != nil {
		return RuntimeError(err)
	}

	token, err := credentials.GetAgentToken(cmd, identity, agenttoken.WithOBO(), agenttoken.WithRefreshTokens(opts.useRefreshTokens))
	if err != nil {
		if isAuthorizationPending(err) {
			fmt.Fprintln(cmd.OutOrStdout(), "Authorization pending.")
			return ExitError{Code: ExitCodeAuthorizationPending, Err: errors.New("authorization pending")}
		}
		return RuntimeError(err)
	}
	if err := credentials.WriteIdentity(ctx, identity); err != nil {
		return RuntimeError(err)
	}

	claims, err := token.Claims()
	if err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), authorizedMessage(identity.Name, claims))
	return nil
}

func authorizedMessage(name string, claims agent.Claims) string {
	msg := fmt.Sprintf("authorized as %s", name)
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
	return msg
}

func printAuthLoginRuntimeContext(cmd *cobra.Command, application *app.App) error {
	ctx := cmd.Context()
	rt, err := application.Runtime()
	if err != nil {
		return err
	}
	status, err := rt.Status(ctx)
	if err != nil {
		return err
	}
	strategyName, err := application.ResolveConsentStrategy(ctx)
	if err != nil {
		return err
	}
	strategy, err := application.ConsentStrategy(agentauth.ConsentMethod(strategyName))
	if err != nil {
		return err
	}
	stdout := cmd.OutOrStdout()
	fmt.Fprintf(stdout, "Runtime end-user proximity: %s (%s)\n", status.EndUserProximity, runtimeEndUserProximitySourceLabel(status.EndUserProximitySource))
	fmt.Fprintf(stdout, "Authorization flow: %s\n", strategy.Guidance().FlowLabel)
	return nil
}

func runtimeEndUserProximitySourceLabel(source cliruntime.EndUserProximitySource) string {
	switch source {
	case cliruntime.EndUserProximitySourceConfigured:
		return "configured"
	case cliruntime.EndUserProximitySourceSavedRuntimeState:
		return "saved runtime state"
	case cliruntime.EndUserProximitySourceAutoDetect:
		return "auto-detect"
	default:
		return string(source)
	}
}

func runAuthComplete(cmd *cobra.Command, factory app.Factory, args []string) error {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	ctx := cmd.Context()
	credentials, err := application.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	pending, exists, err := credentials.GetPendingConsent(ctx)
	if err != nil {
		return RuntimeError(err)
	}
	if !exists {
		return RuntimeError(errors.New("no pending authorization found"))
	}
	if pending.Method == agentauth.ConsentMethodAgentConfirmsIcons {
		if len(args) == 0 {
			return UsageError("auth complete requires icon names for this authorization flow; run `tollbit auth complete <first> <second> <third>`. Valid icon names: %s", strings.Join(pending.IconNames, " "))
		}
	} else if len(args) != 0 {
		return UsageError("auth complete does not accept arguments for this authorization flow")
	}
	strategy, err := application.ConsentStrategy(pending.Method)
	if err != nil {
		return RuntimeError(err)
	}
	if !strategy.SupportsDetachedCompletion() {
		return RuntimeError(errors.New("auth complete is not supported or required for this authorization flow; run auth login instead"))
	}
	resp, err := strategy.CompleteDetached(cmd, pending, agentauth.CompleteDetachedInput{IconNames: args})
	if err != nil {
		if isAuthorizationPending(err) {
			return ExitError{Code: ExitCodeAuthorizationPending, Err: errors.New("authorization still pending")}
		}
		if isUnrecognizedIcon(err) {
			if len(pending.IconNames) > 0 {
				return RuntimeError(fmt.Errorf("%w\nValid icon names: %s", err, strings.Join(pending.IconNames, " ")))
			}
			return RuntimeError(err)
		}
		if isPendingConsentInvalidated(err) {
			if clearErr := credentials.ClearPendingConsent(ctx); clearErr != nil {
				return RuntimeError(fmt.Errorf("clear pending authorization after invalidation: %w", clearErr))
			}
		}
		return RuntimeError(err)
	}
	token, err := credentials.CompletePendingConsent(ctx, pending, resp)
	if err != nil {
		return RuntimeError(err)
	}
	claims, err := token.Claims()
	if err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), authorizedMessage(pending.AgentIdentity.Name, claims))
	return nil
}

func isUnrecognizedIcon(err error) bool {
	var problem problemjson.Problem
	return errors.As(err, &problem) && problem.Code != nil && *problem.Code == problemjson.ErrorCodeUnrecognizedIcon
}

func isAuthorizationPending(err error) bool {
	var pending agentauth.AuthorizationPendingError
	if errors.As(err, &pending) {
		return true
	}
	var problem problemjson.Problem
	return errors.As(err, &problem) && problem.Code != nil && *problem.Code == problemjson.ErrorCodeAuthorizationPending
}

func isPendingConsentInvalidated(err error) bool {
	var problem problemjson.Problem
	if !errors.As(err, &problem) || problem.Code == nil {
		return false
	}
	switch *problem.Code {
	case problemjson.ErrorCodeAccessDenied, problemjson.ErrorCodeExpiredToken, problemjson.ErrorCodeInvalidGrant:
		return true
	default:
		return false
	}
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

	pending, pendingExists, err := credentials.GetPendingConsent(ctx)
	if err != nil {
		return RuntimeError(err)
	}
	refreshStatus := credentials.RefreshTokenStatus(ctx)
	autoRefresh := credentials.AutoRefreshEnabled()

	status := map[string]any{
		"identity": map[string]string{
			"name":       identity.Name,
			"user_agent": identity.UserAgent,
		},
		"auto_refresh":          autoRefresh,
		"pending_authorization": pendingAuthorizationStatus(pending, pendingExists),
		"refresh_token":         refreshStatus,
		"token":                 agenttoken.Status(token, tokenExists, tokenErr),
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
	printRefreshTokenStatus(stdout, refreshStatus, autoRefresh)
	printPendingAuthorizationStatus(stdout, pending, pendingExists)
	if tokenExists && tokenErr == nil {
		if claims, claimsErr := token.Claims(); claimsErr == nil && claims.Subject != "" && claims.Subject != identity.Name {
			fmt.Fprintf(stderr, "token subject %q does not match profile name %q — run 'tollbit auth login'\n", claims.Subject, identity.Name)
		}
	}
	return nil
}

func pendingAuthorizationStatus(pending agentauth.PendingConsent, exists bool) map[string]any {
	status := map[string]any{"pending": exists}
	if !exists {
		return status
	}
	status["challenge_id"] = pending.ChallengeID
	status["identity"] = map[string]string{
		"name":       pending.AgentIdentity.Name,
		"user_agent": pending.AgentIdentity.UserAgent,
	}
	return status
}

func printRefreshTokenStatus(w interface{ Write([]byte) (int, error) }, status agenttoken.RefreshTokenStatus, autoRefresh bool) {
	fmt.Fprintf(w, "Auto-refresh: %s\n", enabledLabel(autoRefresh))
	if status.Error != "" {
		if status.Present {
			fmt.Fprintf(w, "Refresh:    invalid (%s)\n", status.Error)
			return
		}
		fmt.Fprintf(w, "Refresh:    absent (%s)\n", status.Error)
		return
	}
	if !status.Present {
		fmt.Fprintln(w, "Refresh:    absent")
		return
	}
	state := "present"
	if status.Expired {
		state = "expired"
	}
	if status.ExpiresAt != "" {
		fmt.Fprintf(w, "Refresh:    %s (expires %s)\n", state, status.ExpiresAt)
		return
	}
	fmt.Fprintf(w, "Refresh:    %s\n", state)
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func printPendingAuthorizationStatus(w interface{ Write([]byte) (int, error) }, pending agentauth.PendingConsent, exists bool) {
	if !exists {
		return
	}
	fmt.Fprintln(w, "Pending:    authorization pending (complete in browser, then run 'tollbit auth complete')")
	fmt.Fprintf(w, "Pending agent: %s\n", pending.AgentIdentity.Name)
	if pending.AgentIdentity.UserAgent != "" {
		fmt.Fprintf(w, "Pending user agent: %s\n", pending.AgentIdentity.UserAgent)
	}
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
