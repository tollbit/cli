package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetsUnsetVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TOLLBIT_AUTH_BASE_URL=http://oauth.localhost:7011\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "")

	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TOLLBIT_AUTH_BASE_URL"); got != "http://oauth.localhost:7011" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadDoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("TOLLBIT_AUTH_BASE_URL=http://from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "http://already-set")

	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TOLLBIT_AUTH_BASE_URL"); got != "http://already-set" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadStripsQuotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(`TOLLBIT_USER_AGENT="ory-tester"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOLLBIT_USER_AGENT", "")

	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TOLLBIT_USER_AGENT"); got != "ory-tester" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadMissingFileIsNoop(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatal(err)
	}
}
