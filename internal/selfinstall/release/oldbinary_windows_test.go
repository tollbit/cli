//go:build windows

package release

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveOldBinaryDeletesBinaryMovedAside(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	old := exe + oldSuffix
	if err := os.WriteFile(old, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(old) })

	RemoveOldBinary()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be deleted, got %v", old, err)
	}
}
