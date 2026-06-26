package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type guideOptions struct {
	installPath string
}

func NewGuideCommand() *cobra.Command {
	var opts guideOptions
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Print or install the bundled agent guide",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("guide does not accept positional arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuide(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.installPath, "install", "", "install guide to <skills-dir>/<skill-name>/SKILL.md")
	return cmd
}

func runGuide(cmd *cobra.Command, opts guideOptions) error {
	if opts.installPath == "" {
		fmt.Fprint(cmd.OutOrStdout(), skillMarkdown)
		return nil
	}

	skillName, err := skillNameFromMarkdown(skillMarkdown)
	if err != nil {
		return RuntimeError(err)
	}
	destDir := resolveSkillInstallDirectory(opts.installPath, skillName)
	target := filepath.Join(destDir, "SKILL.md")
	return writeGuideFile(target, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func writeGuideFile(path string, stdout, stderr io.Writer) error {
	existed := false
	if _, err := os.Stat(path); err == nil {
		existed = true
	} else if !os.IsNotExist(err) {
		return RuntimeError(fmt.Errorf("stat %s: %v", path, err))
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RuntimeError(fmt.Errorf("create directory for %s: %v", path, err))
	}
	if err := os.WriteFile(path, []byte(skillMarkdown), 0o644); err != nil {
		return RuntimeError(fmt.Errorf("write %s: %v", path, err))
	}

	display := path
	if abs, err := filepath.Abs(path); err == nil {
		display = abs
	}
	if existed {
		fmt.Fprintf(stderr, "overwrote existing SKILL.md at %s\n", display)
	}
	fmt.Fprintf(stdout, "installed SKILL.md at %s\n", display)
	return nil
}
