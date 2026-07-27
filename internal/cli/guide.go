package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/app"
)

type guideOptions struct {
	installPath string
}

func NewGuideCommand(factory app.Factory) *cobra.Command {
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
			return runGuide(cmd, factory, opts)
		},
	}
	cmd.Flags().StringVar(&opts.installPath, "install", "", "install guide to <skills-dir>/<skill-name>/SKILL.md")
	return cmd
}

func runGuide(cmd *cobra.Command, factory app.Factory, opts guideOptions) error {
	markdown, err := renderCLIGuideMarkdown(cmd, factory, skillMarkdown)
	if err != nil {
		return RuntimeError(err)
	}
	if opts.installPath == "" {
		fmt.Fprint(cmd.OutOrStdout(), markdown)
		return nil
	}

	skillName, err := skillNameFromMarkdown(markdown)
	if err != nil {
		return RuntimeError(err)
	}
	destDir := resolveSkillInstallDirectory(opts.installPath, skillName)
	target := filepath.Join(destDir, "SKILL.md")
	return writeGuideFile(target, markdown, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func renderCLIGuideMarkdown(cmd *cobra.Command, factory app.Factory, markdown string) (string, error) {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return "", err
	}
	local, err := application.ConsentStrategy(agentauth.ConsentMethod(factory.Config.Auth.Consent.Strategy.Local))
	if err != nil {
		return "", err
	}
	remote, err := application.ConsentStrategy(agentauth.ConsentMethod(factory.Config.Auth.Consent.Strategy.Remote))
	if err != nil {
		return "", err
	}
	data := cliGuideTemplateData{
		AuthInstructions:   renderAuthInstructions(local.Guidance(), remote.Guidance()),
		AuthCompleteInputs: completeInputs(local, remote),
		AuthRenderedFooter: fmt.Sprintf("Auth instructions rendered for local=%s, remote=%s; re-run `tollbit guide --install <SKILLS_DIR>` if the CLI's auth configuration changes.", local.Method(), remote.Method()),
	}
	tmpl, err := template.New("tollbit-cli-skill").Parse(markdown)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

type cliGuideTemplateData struct {
	AuthInstructions   string
	AuthCompleteInputs string
	AuthRenderedFooter string
}

func renderAuthInstructions(local, remote agentauth.ConsentGuidance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Configured local flow: %s\n\n", local.FlowLabel)
	fmt.Fprintf(&b, "- %s\n", local.LoginInstructions)
	fmt.Fprintf(&b, "- %s\n\n", local.CompleteInstructions)
	fmt.Fprintf(&b, "Configured remote flow: %s\n\n", remote.FlowLabel)
	fmt.Fprintf(&b, "- %s\n", remote.LoginInstructions)
	fmt.Fprintf(&b, "- %s", remote.CompleteInstructions)
	if len(remote.Troubleshooting) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Remote troubleshooting:")
		for _, row := range remote.Troubleshooting {
			fmt.Fprintf(&b, "- `%s`: %s; %s.\n", row.Symptom, row.Meaning, row.Action)
		}
	}
	return strings.TrimSpace(b.String())
}

func completeInputs(local, remote agentauth.ConsentStrategy) string {
	labels := make([]string, 0, 2)
	if local.SupportsDetachedCompletion() {
		labels = append(labels, "local: "+local.Guidance().CompleteArgsLabel)
	}
	if remote.SupportsDetachedCompletion() {
		label := remote.Guidance().CompleteArgsLabel
		if len(labels) > 0 {
			label = "remote: " + label
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return "not used"
	}
	return strings.Join(labels, " / ")
}

func writeGuideFile(path string, markdown string, stdout, stderr io.Writer) error {
	existed := false
	if _, err := os.Stat(path); err == nil {
		existed = true
	} else if !os.IsNotExist(err) {
		return RuntimeError(fmt.Errorf("stat %s: %v", path, err))
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return RuntimeError(fmt.Errorf("create directory for %s: %v", path, err))
	}
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
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
