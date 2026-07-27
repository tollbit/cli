package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cliruntime"
)

type (
	runtimeStatusOptions struct {
		asJSON bool
	}

	runtimeSetOptions struct {
		endUserProximity string
	}
)

func NewRuntimeCommand(factory app.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Manage CLI runtime state",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return UsageError("runtime requires status, set, or help")
			}
			return UsageError("unknown runtime command %q", args[0])
		},
	}
	cmd.AddCommand(
		NewRuntimeStatusCommand(factory),
		NewRuntimeSetCommand(factory),
		NewRuntimeHelpCommand(),
	)
	return cmd
}

func NewRuntimeStatusCommand(factory app.Factory) *cobra.Command {
	var opts runtimeStatusOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show CLI runtime state",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("runtime status does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeStatus(cmd, factory, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print runtime status as JSON")
	return cmd
}

func NewRuntimeSetCommand(factory app.Factory) *cobra.Command {
	var opts runtimeSetOptions
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set CLI runtime state",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("runtime set does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRuntimeSet(cmd, factory, opts)
		},
	}
	cmd.Flags().StringVar(&opts.endUserProximity, "end-user-proximity", "", "runtime end-user proximity to persist: local or remote")
	return cmd
}

func NewRuntimeHelpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help",
		Short: "Explain CLI runtime end-user proximity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("runtime help does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), runtimeHelpText)
			return nil
		},
	}
	return cmd
}

func runRuntimeStatus(cmd *cobra.Command, factory app.Factory, opts runtimeStatusOptions) error {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	rt, err := application.Runtime()
	if err != nil {
		return RuntimeError(err)
	}
	status, err := rt.Status(cmd.Context())
	if err != nil {
		return RuntimeError(err)
	}
	if opts.asJSON {
		out := map[string]any{
			"configured_end_user_proximity": status.ConfiguredEndUserProximity,
			"end_user_proximity_source":     status.EndUserProximitySource,
			"end_user_proximity":            status.EndUserProximity,
			"state_dir":                     status.StateDir,
			"state_path":                    status.StatePath,
			"state_exists":                  status.StateExists,
			"credentials_dir":               application.Config().Credentials.StorageDir,
		}
		if status.StateExists {
			out["saved_end_user_proximity"] = status.SavedEndUserProximity
		}
		return RuntimeError(writeJSON(cmd.OutOrStdout(), out))
	}
	stdout := cmd.OutOrStdout()
	fmt.Fprintf(stdout, "Runtime state dir: %s\n", status.StateDir)
	fmt.Fprintf(stdout, "Runtime state saved: %t\n", status.StateExists)
	fmt.Fprintf(stdout, "Credentials dir: %s\n", application.Config().Credentials.StorageDir)
	fmt.Fprintf(stdout, "End-user proximity: %s (source: %s)\n", status.EndUserProximity, runtimeEndUserProximitySourceLabel(status.EndUserProximitySource))
	return nil
}

func runRuntimeSet(cmd *cobra.Command, factory app.Factory, opts runtimeSetOptions) error {
	if !cmd.Flags().Changed("end-user-proximity") {
		return UsageError("runtime set requires --end-user-proximity")
	}
	proximity, err := parseRuntimeSetEndUserProximity(opts.endUserProximity)
	if err != nil {
		return UsageError("%s", err.Error())
	}
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	rt, err := application.Runtime()
	if err != nil {
		return RuntimeError(err)
	}
	if err := rt.SetEndUserProximity(cmd.Context(), proximity); err != nil {
		return RuntimeError(err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "saved runtime end-user proximity %s\n", proximity)
	return nil
}

func parseRuntimeSetEndUserProximity(value string) (cliruntime.EndUserProximity, error) {
	switch strings.TrimSpace(value) {
	case string(cliruntime.EndUserProximityLocal):
		return cliruntime.EndUserProximityLocal, nil
	case string(cliruntime.EndUserProximityRemote):
		return cliruntime.EndUserProximityRemote, nil
	case "", "auto-detect":
		return "", errors.New("runtime set --end-user-proximity must be local or remote")
	default:
		return "", fmt.Errorf("runtime set --end-user-proximity must be %q or %q", cliruntime.EndUserProximityLocal, cliruntime.EndUserProximityRemote)
	}
}

const runtimeHelpText = `Runtime end-user proximity describes where the agent environment running the CLI
is located relative to the end user's browser.

Terms:
  end user:
    The human who asked the agent to do work and can complete browser consent.

  agent:
    The AI system acting for the end user, including the model, harness, tool
    runner, and environment invoking the tollbit CLI.

  end-user proximity:
    Whether the agent environment running the tollbit CLI is local or remote
    relative to the end user's browser.

Values:
  local:
    The CLI and the end user's browser are available in the same environment.

  remote:
    The CLI is running in an agent environment separate from the end user's
    browser. The agent may need to relay printed browser instructions to the end
    user, then run auth complete after browser approval.

If end-user proximity is not configured, the CLI attempts to determine it
automatically.

Guidance:
  Prefer runtime set when the agent environment is known. Persisting end-user
  proximity lets future commands choose the right authorization flow without
  repeating --end-user-proximity on each command. Use the flag only for a
  one-off override.

Commands:
  tollbit runtime status
  tollbit runtime set --end-user-proximity local
  tollbit runtime set --end-user-proximity remote

Overrides:
  --end-user-proximity local|remote overrides end-user proximity for one invocation.
  TOLLBIT_RUNTIME_END_USER_PROXIMITY=local|remote sets it from the environment.
  TOLLBIT_RUNTIME_STATE_DIR selects where runtime state is read or written.
`
