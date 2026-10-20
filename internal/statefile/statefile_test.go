package statefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")

	if err := WriteAtomic(context.Background(), path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(context.Background(), path, []byte("second")); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("expected the replaced content, got %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("expected mode 0600, got %#o", mode)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected no leftover temp files, got %d entries", len(entries))
	}
}

func TestWriteAtomicHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "state.json")

	if err := WriteAtomic(ctx, path, []byte("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected no file to be written")
	}
}
