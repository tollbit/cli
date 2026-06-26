package main

import (
	"context"
	"fmt"
	"os"

	tollbitcli "github.com/tollbit/tollbit-cli"
	"github.com/tollbit/tollbit-cli/internal/app"
	"github.com/tollbit/tollbit-cli/internal/cli"
	"github.com/tollbit/tollbit-cli/internal/configuration"
	"github.com/tollbit/tollbit-cli/internal/envfile"
	"github.com/tollbit/tollbit-cli/internal/logging"
)

func main() {
	if err := envfile.LoadDefault(); err != nil {
		fmt.Fprintln(os.Stderr, "load .env:", err)
		os.Exit(1)
	}

	ctx, cleanup, err := logging.NewContext(context.Background(), os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer cleanup()

	config, err := configuration.AssembleConfiguration(tollbitcli.DefaultConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd := cli.NewCommandTree(app.Factory{Config: config})
	rootCmd.SetArgs(os.Args[1:])
	rootCmd.SetIn(os.Stdin)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), err)
		os.Exit(cli.ExitCode(err))
	}
}
