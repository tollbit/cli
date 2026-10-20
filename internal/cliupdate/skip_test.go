package cliupdate

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSkip(t *testing.T) {
	root := &cobra.Command{Use: "tollbit"}
	runtime := &cobra.Command{Use: "runtime"}
	runtime.AddCommand(&cobra.Command{Use: "status"}, &cobra.Command{Use: "help"})
	completion := &cobra.Command{Use: "completion"}
	completion.AddCommand(&cobra.Command{Use: "bash"})
	root.AddCommand(
		runtime,
		completion,
		&cobra.Command{Use: "version"},
		&cobra.Command{Use: "help"},
		&cobra.Command{Use: cobra.ShellCompRequestCmd},
		&cobra.Command{Use: cobra.ShellCompNoDescRequestCmd},
	)

	tests := []struct {
		args []string
		want bool
	}{
		{args: nil, want: true},
		{args: []string{"version"}, want: true},
		{args: []string{"help"}, want: true},
		{args: []string{"runtime", "help"}, want: true},
		{args: []string{"completion", "bash"}, want: true},
		{args: []string{cobra.ShellCompRequestCmd}, want: true},
		{args: []string{cobra.ShellCompNoDescRequestCmd}, want: true},
		{args: []string{"runtime", "status"}, want: false},
		{args: []string{"runtime"}, want: false},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			cmd, _, err := root.Find(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if got := Skip(cmd); got != tt.want {
				t.Fatalf("Skip(%q) = %t, want %t", cmd.CommandPath(), got, tt.want)
			}
		})
	}
}
