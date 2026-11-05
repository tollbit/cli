package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var repoRoot = filepath.Join("..", "..", "..")

func TestPromptFilesAreUpToDate(t *testing.T) {
	files, err := Files(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(repoRoot, p))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is out of date: run `make prompts` and commit the result", p)
		}
	}
}

func TestFillSetsTheRelease(t *testing.T) {
	got, err := fill("t.md", "releases/download/v{{version}}/tollbit_{{version}}_checksums.txt", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if want := "releases/download/v1.2.3/tollbit_1.2.3_checksums.txt"; string(got) != want {
		t.Fatalf("fill = %q, want %q", got, want)
	}
}

func TestFillRejectsAnUnknownPlaceholder(t *testing.T) {
	if _, err := fill("t.md", "Hello {{name}}", "1.2.3"); err == nil || !strings.Contains(err.Error(), "t.md") {
		t.Fatalf("fill error = %v, want an unknown placeholder error naming t.md", err)
	}
}
