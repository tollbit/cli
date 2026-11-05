package prompts

import (
	"strings"
	"testing"
)

const testTemplate = `Intro.

{{hints_table}}

## Install

### Install: script

### Install: Claude plugin

### Install: MCP (coming soon)

### Install: not supported

Use https://raw.githubusercontent.com/tollbit/cli/v{{version}}/scripts/install.sh --version v{{version}}

## TODO

Not published.
`

func TestRenderMergesPlatformsWithTheSameHint(t *testing.T) {
	m := platformList{Platforms: []platform{
		{Name: "Chat (desktop)", Detection: "`X=1`", Install: "claude-plugin"},
		{Name: "Chat (web)", Detection: "`X=1`", Install: "claude-plugin"},
		{Name: "Terminal", Detection: "`Y=1`", Install: "script"},
		{Name: "Untested", Install: "script"},
	}}
	files, err := render([]byte(testTemplate), m, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(files[PromptPath])
	for _, want := range []string{
		"   | Chat (desktop) or Chat (web) | `X=1` | Install: Claude plugin |\n",
		"   | Terminal | `Y=1` | Install: script |\n",
		"   | Any other agent with no shell | only your instructions | Install: not supported |",
		"raw.githubusercontent.com/tollbit/cli/v1.2.3/scripts/install.sh",
		"--version v1.2.3",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, prompt)
		}
	}
	for _, unwanted := range []string{"Untested", "## TODO", "Not published.", "{{"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt has %q:\n%s", unwanted, prompt)
		}
	}
}

func TestRenderRejectsAnUnknownInstallValue(t *testing.T) {
	m := platformList{Platforms: []platform{{Name: "Agent", Detection: "`Z=1`", Install: "brew"}}}
	if _, err := render([]byte(testTemplate), m, "1.2.3"); err == nil || !strings.Contains(err.Error(), `install "brew"`) {
		t.Fatalf("render error = %v, want an unknown install error", err)
	}
}

func TestRenderRejectsATemplateWithoutAnInstallSection(t *testing.T) {
	template := strings.Replace(testTemplate, "### Install: MCP (coming soon)\n", "", 1)
	if _, err := render([]byte(template), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "MCP (coming soon)") {
		t.Fatalf("render error = %v, want a missing section error", err)
	}
}

func TestRenderRejectsAnUnknownPlaceholder(t *testing.T) {
	template := strings.Replace(testTemplate, "Intro.", "Intro {{name}}.", 1)
	if _, err := render([]byte(template), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("render error = %v, want an unknown placeholder error", err)
	}
}
