package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	tollbitcli "github.com/tollbit/cli"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/cli"
	"github.com/tollbit/cli/internal/cliupdate"
	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/envfile"
	"github.com/tollbit/cli/internal/logging"
	"github.com/tollbit/cli/internal/selfinstall/release"
)

func main() {
	release.RemoveOldBinary()

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

	factory := app.NewFactory(config, os.Stderr)
	rootCmd := cli.NewCommandTree(factory)
	rootCmd.SetArgs(os.Args[1:])
	rootCmd.SetIn(os.Stdin)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		var restart cliupdate.RestartError
		if errors.As(err, &restart) {
			_ = cleanup()
			code, err := restart.Restart()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(code)
		}
		factory.Process.RecordUpdateRequired(ctx, err)
		fmt.Fprintln(rootCmd.ErrOrStderr(), cli.RenderError(err))
		os.Exit(cli.ExitCode(err))
	}
}
