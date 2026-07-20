//go:build dev

package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleConfigurationMergesDevelopmentConfig(t *testing.T) {
	dir := t.TempDir()
	writeDevelopmentConfig(t, dir, "gateway:\n  base_url: https://gateway-development.example.com\n")

	config := assembleTestConfiguration(t, dir)

	if config.Gateway.BaseURL != "https://gateway-development.example.com" {
		t.Fatalf("expected gateway development override, got %q", config.Gateway.BaseURL)
	}
}

func TestAssembleConfigurationEnvOverridesFiles(t *testing.T) {
	dir := t.TempDir()
	writeDevelopmentConfig(t, dir, "gateway:\n  base_url: https://gateway-development.example.com\n")
	t.Setenv("TOLLBIT_GATEWAY_BASE_URL", "https://gateway-env.example.com")

	config := assembleTestConfiguration(t, dir)

	if config.Gateway.BaseURL != "https://gateway-env.example.com" {
		t.Fatalf("expected gateway env override, got %q", config.Gateway.BaseURL)
	}
}

func writeDevelopmentConfig(t *testing.T, dir string, content string) {
	t.Helper()
	path := filepath.Join(dir, developmentConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
