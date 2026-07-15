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

func TestLoadIgnoresNonTollbitKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\nTOLLBIT_USER_AGENT=x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOO", "")
	t.Setenv("TOLLBIT_USER_AGENT", "")

	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("FOO"); got != "" {
		t.Fatalf("non-TOLLBIT_ key was set: FOO=%q", got)
	}
	if got := os.Getenv("TOLLBIT_USER_AGENT"); got != "x" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadDefaultNoEnvFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOLLBIT_AUTH_BASE_URL=http://from-cwd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(EnvFilePathVar, "")
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "")

	if err := LoadDefault(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TOLLBIT_AUTH_BASE_URL"); got != "" {
		t.Fatalf("cwd .env was loaded without %s set: %q", EnvFilePathVar, got)
	}
}

func TestLoadDefaultUsesEnvFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.env")
	if err := os.WriteFile(path, []byte("TOLLBIT_AUTH_BASE_URL=http://from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvFilePathVar, path)
	t.Setenv("TOLLBIT_AUTH_BASE_URL", "")

	if err := LoadDefault(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TOLLBIT_AUTH_BASE_URL"); got != "http://from-file" {
		t.Fatalf("got %q", got)
	}
}
