package prompts

import (
	"strings"
	"testing"
)

const testTemplate = `Intro.

{{hints_table}}

## Install

### Install: script

### Install: Codex

### Install: Claude plugin

### Install: MCP (coming soon)

### Install: not supported

Use https://raw.githubusercontent.com/tollbit/cli/v{{version}}/scripts/install.sh --version v{{version}}

## Troubleshoot a failed install

{{troubleshooting}}

## TODO

Not published.
`

const testTroubleshoot = `# Diagnose a failed install

Work out where it breaks.

## 1. Environment

  ` + "```bash" + `
  # not a heading
  curl https://github.com/tollbit/cli/releases/download/v{{version}}/sums.txt
  ` + "```" + `

## 2. Report
`

func TestRenderMergesPlatformsWithTheSameHint(t *testing.T) {
	m := platformList{Platforms: []platform{
		{Name: "Chat (desktop)", Detection: "`X=1`", Install: "claude-plugin"},
		{Name: "Chat (web)", Detection: "`X=1`", Install: "claude-plugin"},
		{Name: "Terminal", Detection: "`Y=1`", Install: "script"},
		{Name: "Untested", Install: "script"},
	}}
	files, err := render([]byte(testTemplate), []byte(testTroubleshoot), m, "1.2.3")
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
	if _, err := render([]byte(testTemplate), []byte(testTroubleshoot), m, "1.2.3"); err == nil || !strings.Contains(err.Error(), `install "brew"`) {
		t.Fatalf("render error = %v, want an unknown install error", err)
	}
}

func TestRenderRejectsATemplateWithoutAnInstallSection(t *testing.T) {
	template := strings.Replace(testTemplate, "### Install: MCP (coming soon)\n", "", 1)
	if _, err := render([]byte(template), []byte(testTroubleshoot), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "MCP (coming soon)") {
		t.Fatalf("render error = %v, want a missing section error", err)
	}
}

func TestRenderRejectsAnUnknownPlaceholder(t *testing.T) {
	template := strings.Replace(testTemplate, "Intro.", "Intro {{name}}.", 1)
	if _, err := render([]byte(template), []byte(testTroubleshoot), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("render error = %v, want an unknown placeholder error", err)
	}
}

func TestRenderEmbedsTheTroubleshootingPrompt(t *testing.T) {
	files, err := render([]byte(testTemplate), []byte(testTroubleshoot), platformList{}, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(files[PromptPath])
	want := "## Troubleshoot a failed install\n\n" +
		"Work out where it breaks.\n\n" +
		"### 1. Environment\n\n" +
		"  ```bash\n" +
		"  # not a heading\n" +
		"  curl https://github.com/tollbit/cli/releases/download/v1.2.3/sums.txt\n" +
		"  ```\n\n" +
		"### 2. Report\n"
	if !strings.HasSuffix(prompt, want) {
		t.Errorf("prompt doesn't end with the embedded troubleshooting steps:\n%s", prompt)
	}
	if strings.Contains(prompt, "Diagnose a failed install") {
		t.Errorf("prompt has the troubleshooting prompt's title:\n%s", prompt)
	}
}

func TestRenderRejectsATemplateWithoutTheTroubleshootingPlaceholder(t *testing.T) {
	template := strings.Replace(testTemplate, "{{troubleshooting}}\n", "", 1)
	if _, err := render([]byte(template), []byte(testTroubleshoot), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "{{troubleshooting}}") {
		t.Fatalf("render error = %v, want a missing placeholder error", err)
	}
}

func TestRenderRejectsATroubleshootingPromptWithoutATitle(t *testing.T) {
	if _, err := render([]byte(testTemplate), []byte("No title.\n"), platformList{}, "1.2.3"); err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("render error = %v, want a missing title error", err)
	}
}
