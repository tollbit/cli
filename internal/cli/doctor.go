package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tollbit/cli/internal/app"
	"github.com/tollbit/cli/internal/doctor"
	"github.com/tollbit/cli/internal/version"
	"github.com/tollbit/cli/skill"
)

type doctorOptions struct {
	asJSON    bool
	skillsDir string
}

func NewDoctorCommand(factory app.Factory) *cobra.Command {
	var opts doctorOptions
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check that the CLI can run, write its files and reach TollBit",
		Long: "Check that the CLI can run on this machine: where the binary is and whether it is on PATH, " +
			"whether the CLI can write its state and credentials directories, and whether every host it calls " +
			"resolves, completes TLS and answers through this network (including any proxy). " +
			"It also lists the agent skills bundled with this CLI and their version, to compare with an installed SKILL.md. " +
			"Pass --skills-dir with the folder you gave `tollbit guide --install` to check the installed copies instead: " +
			"a missing or out-of-date skill warns but does not fail. Skills from the TollBit plugin live elsewhere and are not checked.\n\n" +
			"Exits 1 when a check fails. Never prints tokens or credentials.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return UsageError("doctor does not accept arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("skills-dir") && strings.TrimSpace(opts.skillsDir) == "" {
				return UsageError("--skills-dir needs a folder")
			}
			return runDoctor(cmd, factory, opts, doctor.Options{})
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, "json", false, "print the report as JSON")
	cmd.Flags().StringVar(&opts.skillsDir, "skills-dir", "", "check the skills installed in this folder, the one given to `tollbit guide --install`")
	return cmd
}

// runDoctor fills in the configuration and credentials of base and runs
// doctor; tests pass the machine seams in base.
func runDoctor(cmd *cobra.Command, factory app.Factory, opts doctorOptions, base doctor.Options) error {
	application, err := appForCommand(factory, cmd)
	if err != nil {
		return RuntimeError(err)
	}
	credentials, err := application.Credentials()
	if err != nil {
		return RuntimeError(err)
	}
	base.Config = application.Config()
	base.Credentials = credentials
	base.Skills, err = doctorSkills()
	if err != nil {
		return RuntimeError(err)
	}
	base.SkillsDir = opts.skillsDir
	if abs, err := filepath.Abs(opts.skillsDir); err == nil && opts.skillsDir != "" {
		base.SkillsDir = abs
	}

	report := doctor.Run(cmd.Context(), base)
	if opts.asJSON {
		if err := writeJSON(cmd.OutOrStdout(), report); err != nil {
			return RuntimeError(err)
		}
	} else if err := printDoctorReport(cmd.OutOrStdout(), report, opts); err != nil {
		return RuntimeError(err)
	}
	if !report.OK {
		return ExitError{Code: 1, Err: fmt.Errorf("doctor: %d check(s) failed", countStatus(report, doctor.StatusFail))}
	}
	return nil
}

// doctorGroupTitles orders the report; a note prints once under its title.
// The skills note is about comparing by hand, so --skills-dir drops it.
var doctorGroupTitles = []struct{ key, title, note string }{
	{doctor.GroupExecution, "Execution", ""},
	{doctor.GroupDirectories, "Directories", ""},
	{doctor.GroupNetwork, "Network", ""},
	{doctor.GroupAuth, "Auth", ""},
	{doctor.GroupSkills, "Skills", "  Compare with the version: line of your installed SKILL.md. <skills-dir> is the folder that holds your skill folders " +
		"(for example ~/.claude/skills). If the skill came from the TollBit plugin, update the plugin instead."},
}

// doctorSkills lists the bundled skills by the name in their frontmatter.
// Each renders with this CLI's version.
func doctorSkills() ([]doctor.Skill, error) {
	var skills []doctor.Skill
	for _, flag := range []string{"cli", "analytics"} {
		markdown, err := bundledSkillMarkdown(flag)
		if err != nil {
			return nil, err
		}
		name, err := skill.Name(markdown)
		if err != nil {
			return nil, err
		}
		skills = append(skills, doctor.Skill{
			Name:      name,
			Version:   version.Version,
			GuideName: flag,
		})
	}
	return skills, nil
}

// printDoctorReport uses plain-text status tags rather than symbols so the
// report survives legacy Windows consoles and copy-paste into chat. Hints sit
// under the detail column.
func printDoctorReport(w io.Writer, report doctor.Report, opts doctorOptions) error {
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, group := range doctorGroupTitles {
		printed := false
		for _, c := range report.Checks {
			if c.Group != group.key {
				continue
			}
			if !printed {
				fmt.Fprintln(table, group.title)
				if group.note != "" && opts.skillsDir == "" {
					fmt.Fprintln(table, group.note)
				}
				printed = true
			}
			fmt.Fprintf(table, "  %s\t%s\t%s\n", strings.ToUpper(string(c.Status)), c.Name, c.Detail)
			if c.Hint != "" {
				fmt.Fprintf(table, "\t\thint: %s\n", c.Hint)
			}
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	fails, warns := countStatus(report, doctor.StatusFail), countStatus(report, doctor.StatusWarn)
	switch {
	case fails > 0:
		fmt.Fprintf(w, "\n%d failed, %d warning(s). Share this output (or `tollbit doctor --json`) with TollBit support if you need help.\n", fails, warns)
	case warns > 0:
		fmt.Fprintf(w, "\nAll required checks passed, %d warning(s).\n", warns)
	default:
		fmt.Fprintln(w, "\nAll checks passed.")
	}
	return nil
}

func countStatus(report doctor.Report, status doctor.Status) int {
	n := 0
	for _, c := range report.Checks {
		if c.Status == status {
			n++
		}
	}
	return n
}
