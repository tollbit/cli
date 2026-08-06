package cliruntime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tollbit/cli/internal/configuration"
)

func TestEndUserProximityUsesConfiguredValueBeforeState(t *testing.T) {
	dir := t.TempDir()
	rt, err := New(Config{ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityLocal, StateDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.SetEndUserProximity(context.Background(), EndUserProximityRemote); err != nil {
		t.Fatal(err)
	}

	got, err := rt.EndUserProximity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != EndUserProximityLocal {
		t.Fatalf("expected configured end-user proximity to override state, got %q", got)
	}
}

func TestAutoDetectReadsRuntimeState(t *testing.T) {
	rt, err := New(Config{ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityAutoDetect, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.SetEndUserProximity(context.Background(), EndUserProximityLocal); err != nil {
		t.Fatal(err)
	}

	got, err := rt.EndUserProximity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != EndUserProximityLocal {
		t.Fatalf("expected end-user proximity from runtime state, got %q", got)
	}
}

func TestAutoDetectDefaultsLocalWithoutRuntimeState(t *testing.T) {
	rt, err := New(Config{ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityAutoDetect, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	got, err := rt.EndUserProximity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != EndUserProximityLocal {
		t.Fatalf("expected local fallback, got %q", got)
	}
}

func TestStatusReportsEndUserProximitySource(t *testing.T) {
	tests := []struct {
		name            string
		configured      string
		saved           EndUserProximity
		wantProximity   EndUserProximity
		wantSaved       EndUserProximity
		wantSource      EndUserProximitySource
		wantStateExists bool
	}{
		{
			name:            "configured local overrides saved state",
			configured:      configuration.RuntimeEndUserProximityLocal,
			saved:           EndUserProximityRemote,
			wantProximity:   EndUserProximityLocal,
			wantSaved:       EndUserProximityRemote,
			wantSource:      EndUserProximitySourceConfigured,
			wantStateExists: true,
		},
		{
			name:            "configured remote",
			configured:      configuration.RuntimeEndUserProximityRemote,
			wantProximity:   EndUserProximityRemote,
			wantSource:      EndUserProximitySourceConfigured,
			wantStateExists: false,
		},
		{
			name:            "auto detect reads saved runtime state",
			configured:      configuration.RuntimeEndUserProximityAutoDetect,
			saved:           EndUserProximityLocal,
			wantProximity:   EndUserProximityLocal,
			wantSaved:       EndUserProximityLocal,
			wantSource:      EndUserProximitySourceSavedRuntimeState,
			wantStateExists: true,
		},
		{
			name:            "auto detect defaults local without saved state",
			configured:      configuration.RuntimeEndUserProximityAutoDetect,
			wantProximity:   EndUserProximityLocal,
			wantSource:      EndUserProximitySourceAutoDetect,
			wantStateExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, err := New(Config{ConfiguredEndUserProximity: tt.configured, StateDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			if tt.saved != "" {
				if err := rt.SetEndUserProximity(context.Background(), tt.saved); err != nil {
					t.Fatal(err)
				}
			}
			status, err := rt.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if status.EndUserProximity != tt.wantProximity || status.EndUserProximitySource != tt.wantSource || status.StateExists != tt.wantStateExists || status.SavedEndUserProximity != tt.wantSaved {
				t.Fatalf("unexpected status: %#v", status)
			}
		})
	}
}

func TestSetEndUserProximityWritesRuntimeStateSecurely(t *testing.T) {
	dir := t.TempDir()
	rt, err := New(Config{ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityAutoDetect, StateDir: dir})
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.SetEndUserProximity(context.Background(), EndUserProximityRemote); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, stateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected runtime state mode 0600, got %#o", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, stateFilename))
	if err != nil {
		t.Fatal(err)
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.EndUserProximity != EndUserProximityRemote || st.UpdatedAt == "" {
		t.Fatalf("unexpected runtime state: %#v", st)
	}
}

func TestSetEndUserProximityRejectsAutoDetect(t *testing.T) {
	rt, err := New(Config{ConfiguredEndUserProximity: configuration.RuntimeEndUserProximityAutoDetect, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	if err := rt.SetEndUserProximity(context.Background(), EndUserProximity("auto-detect")); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateBrowserURL(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"http://x", "https://x"} {
		if err := validateBrowserURL(rawURL); err != nil {
			t.Fatalf("expected %q to be allowed, got %v", rawURL, err)
		}
	}

	for _, rawURL := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"custom://x",
		"",
		"example.com/path",
	} {
		if err := validateBrowserURL(rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}
