package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/client/auth"
)

type identitySetOptions struct {
	userAgent string
}

type identityGetOptions struct {
	asJSON bool
}

func NewIdentityCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "identity",
		Short: "Manage the persisted agent identity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("identity requires set, get, or clear")
			}
			return UsageError("unknown identity command %q", args[0])
		},
	}

	cmd.AddCommand(
		NewIdentitySetCommand(factory),
		NewIdentityGetCommand(factory),
		NewIdentityClearCommand(factory),
	)
	return cmd
}

func NewIdentitySetCommand(factory app.Factory) *cobra.Command {
	opts := identitySetOptions{
		userAgent: factory.Config.Agent.DefaultUserAgent,
	}
	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Save the agent identity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return UsageError("identity set requires exactly one name")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentitySet(cmd, factory, opts, args[0])
		},
	}
	cmd.Flags().StringVar(&opts.userAgent, "user-agent", opts.userAgent, "agent identity user agent")
	return cmd
}

func NewIdentityGetCommand(factory app.Factory) *cobra.Command {
	var opts identityGetOptions
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show the agent identity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("identity get does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityGet(cmd, factory, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "emit raw JSON")
	return cmd
}

func NewIdentityClearCommand(factory app.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the agent identity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("identity clear does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIdentityClear(cmd, factory)
		},
	}
}

func runIdentitySet(cmd *cobra.Command, factory app.Factory, opts identitySetOptions, name string) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	identity := auth.AgentIdentity{Name: name, UserAgent: opts.userAgent}
	if err := credentials.SaveIdentity(cmd.Context(), identity); err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "saved agent identity %s\n", strings.TrimSpace(name))
	return nil
}

func runIdentityGet(cmd *cobra.Command, factory app.Factory, opts identityGetOptions) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	identity, err := credentials.GetIdentity(cmd.Context())
	if err != nil {
		return RuntimeError(err)
	}
	stdout := cmd.OutOrStdout()
	if opts.asJSON {
		return RuntimeError(writeJSON(stdout, identity))
	}
	fmt.Fprintf(stdout, "name: %s\n", identity.Name)
	if identity.UserAgent != "" {
		fmt.Fprintf(stdout, "user-agent: %s\n", identity.UserAgent)
	}
	return nil
}

func runIdentityClear(cmd *cobra.Command, factory app.Factory) error {
	app, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := app.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	if err := credentials.ClearIdentity(cmd.Context()); err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "cleared agent identity")
	return nil
}
