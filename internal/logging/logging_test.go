package logging

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewContextLogging(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		wantLog   bool
		wantError string
	}{
		{name: "unset disables logs"},
		{name: "disabled emits nothing", level: "disabled"},
		{name: "debug emits logs", level: "debug", wantLog: true},
		{name: "invalid level errors", level: "verbose", wantError: "invalid TOLLBIT_LOG_LEVEL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(LevelEnvVar, tt.level)
			t.Setenv(OutputEnvVar, "")
			var out bytes.Buffer
			ctx, cleanup, err := NewContext(context.Background(), &out)
			if tt.wantError != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := cleanup(); err != nil {
				t.Fatal(err)
			}

			zerolog.Ctx(ctx).Debug().Str("component", "test").Msg("hello")
			if tt.wantLog && !strings.Contains(out.String(), "hello") {
				t.Fatalf("expected log output, got %q", out.String())
			}
			if !tt.wantLog && out.Len() != 0 {
				t.Fatalf("expected no log output, got %q", out.String())
			}
		})
	}
}

func TestNewContextWritesToFile(t *testing.T) {
	t.Setenv(LevelEnvVar, "debug")
	path := filepath.Join(t.TempDir(), "tollbit.log")
	t.Setenv(OutputEnvVar, path)

	ctx, cleanup, err := NewContext(context.Background(), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	zerolog.Ctx(ctx).Debug().Msg("file-log")
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "file-log") {
		t.Fatalf("expected file log output, got %q", string(b))
	}
}
