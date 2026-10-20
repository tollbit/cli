// Package statefile writes the small files the CLI keeps between runs, such
// as runtime.json and cached server responses, so a crash or a concurrent
// run never leaves one half-written.
package statefile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic replaces path with content, readable only by the current user.
// It writes a temp file beside path and renames it over path, so readers see
// either the old content or the new, never a partial write. Missing parent
// directories are created.
func WriteAtomic(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("set permissions on temp file for %s: %w", path, err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
