package selfinstall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	lockFilename   = "self-install.lock"
	lockPoll       = 250 * time.Millisecond
	takeoverSuffix = ".takeover"
)

// lockWait is how long to wait for another process's install to finish. It
// is a var so tests can shorten it.
var lockWait = 90 * time.Second

// acquireLock creates the lock file at path, waiting up to wait while another
// process holds it. A lock older than stale is left over from a process that
// died mid-install and is taken over. The returned func releases the lock.
func acquireLock(ctx context.Context, path string, wait, stale time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	deadline := time.Now().Add(wait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create lock file: %w", err)
		}
		if clearStaleLock(path, stale) {
			continue
		}
		if time.Now().After(deadline) {
			return nil, errors.New("another update is still in progress")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockPoll):
		}
	}
}

func clearStaleLock(path string, stale time.Duration) bool {
	if !isOlderThan(path, stale) {
		return false
	}
	takeover := path + takeoverSuffix
	f, err := os.OpenFile(takeover, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if isOlderThan(takeover, stale) {
			_ = os.Remove(takeover)
		}
		return false
	}
	_ = f.Close()
	defer func() { _ = os.Remove(takeover) }()

	return isOlderThan(path, stale) && os.Remove(path) == nil
}

func isOlderThan(path string, age time.Duration) bool {
	info, err := os.Stat(path)
	return err == nil && time.Since(info.ModTime()) > age
}
