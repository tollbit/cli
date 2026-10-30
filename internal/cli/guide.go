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
	"github.com/tollbit/cli/internal/version"
)

type guideOptions struct {
	installPath string
	skill       string
}

func NewGuideCommand(factory app.Factory) *cobra.Command {
	var opts guideOptions
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Print or install bundled agent skills",
		Long: "Print or install bundled skills for agent harnesses.\n\n" +
			"Use `--skill cli` (the default) for searching, pricing and fetching content.\n" +
			"Use `--skill analytics` for querying your organization's site analytics.",
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
	cmd.Flags().StringVar(&opts.skill, "skill", "cli", "bundled skill to print or install: cli, analytics")
	return cmd
}

func runGuide(cmd *cobra.Command, factory app.Factory, opts guideOptions) error {
	markdown, err := bundledSkillMarkdown(opts.skill)
	if err != nil {
		return UsageError("%s", err.Error())
	}
	if isCLIGuideSkill(opts.skill) {
		markdown, err = renderCLIGuideMarkdown(cmd, factory, markdown)
	} else {
		markdown, err = renderSkillTemplate(opts.skill, markdown, skillTemplateData{Version: version.Version})
	}
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

// isCLIGuideSkill reports whether name selects the tollbit-cli skill, the only
// bundled skill that also needs auth details rendered into it.
func isCLIGuideSkill(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "cli", "consumer", "tollbit-cli":
		return true
	default:
		return false
	}
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
		skillTemplateData:  skillTemplateData{Version: version.Version},
		AuthInstructions:   renderAuthInstructions(local.Guidance(), remote.Guidance()),
		AuthCompleteInputs: completeInputs(local, remote),
		AuthRenderedFooter: fmt.Sprintf("Auth instructions rendered for local=%s, remote=%s; re-run `tollbit guide --install <SKILLS_DIR>` if the CLI's auth configuration changes.", local.Method(), remote.Method()),
	}
	return renderSkillTemplate("tollbit-cli", markdown, data)
}

// renderSkillTemplate renders a bundled skill; every skill takes at least
// {{ .Version }} (the CLI version) for its frontmatter.
func renderSkillTemplate(name, markdown string, data any) (string, error) {
	tmpl, err := template.New(name + "-skill").Parse(markdown)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}

type (
	skillTemplateData struct {
		Version string
	}

	cliGuideTemplateData struct {
		skillTemplateData
		AuthInstructions   string
		AuthCompleteInputs string
		AuthRenderedFooter string
	}
)

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

func bundledSkillMarkdown(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "cli", "consumer", "tollbit-cli":
		return skillMarkdown, nil
	case "analytics", "tollbit-analytics":
		return analyticsSkillMarkdown, nil
	default:
		return "", fmt.Errorf("unknown guide skill %q; expected cli or analytics", name)
	}
}
