// Package release replaces the CLI binary with a published release. It
// follows the same steps as scripts/install.sh: download the release archive
// and its checksums, verify the archive's SHA-256, extract the binary, and
// move it into place.
package release

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/selfinstall"
)

const (
	// DownloadTimeout bounds downloading the archive and its checksums.
	DownloadTimeout = 60 * time.Second

	binaryName = "tollbit"
)

// Installer replaces a CLI binary with a release from BaseURL. The zero value
// of each optional field picks the running process's value.
type Installer struct {
	// BaseURL holds one directory per release tag (v<version>) with the
	// archives and checksums GoReleaser publishes.
	BaseURL string
	// LockDir holds the lock file that keeps two processes from installing
	// at once.
	LockDir string
	// Executable is the binary to replace; os.Executable() when empty.
	// Symlinks are followed so the real file is replaced.
	Executable string
	// HTTPClient downloads the release; http.DefaultClient when nil.
	HTTPClient *http.Client

	goos   string
	goarch string
}

// Install replaces the binary with release target, unless the binary on disk
// is already at least that new. current is the running version.
func (i Installer) Install(ctx context.Context, current, target string) (selfinstall.Result, error) {
	if err := selfinstall.CheckNewer(current, target); err != nil {
		return selfinstall.Result{}, err
	}
	exe, err := selfinstall.Executable(i.Executable)
	if err != nil {
		return selfinstall.Result{}, err
	}
	if err := selfinstall.CheckWritable(filepath.Dir(exe)); err != nil {
		return selfinstall.Result{}, err
	}
	return selfinstall.InstallLocked(ctx, i.LockDir, exe, target, func(ctx context.Context) error {
		return i.download(ctx, exe, target)
	})
}

// download fetches, verifies and extracts release target and moves it over
// exe.
func (i Installer) download(ctx context.Context, exe, target string) error {
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "tollbit-update-*")
	if err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	asset := i.asset(target)
	releaseURL := strings.TrimRight(i.BaseURL, "/") + "/v" + target + "/"
	checksums, err := i.fetchChecksums(ctx, releaseURL+asset.checksums)
	if err != nil {
		return err
	}
	want, ok := checksums[asset.archive]
	if !ok {
		return fmt.Errorf("checksum entry not found for %s", asset.archive)
	}
	archive := filepath.Join(tmpDir, asset.archive)
	got, err := i.fetchFile(ctx, releaseURL+asset.archive, archive)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s", asset.archive)
	}

	staged, err := extractBinary(archive, asset, filepath.Dir(exe))
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(staged) }()
	return replace(staged, exe, i.goosOrDefault())
}

func (i Installer) goosOrDefault() string {
	if i.goos != "" {
		return i.goos
	}
	return runtime.GOOS
}

func (i Installer) goarchOrDefault() string {
	if i.goarch != "" {
		return i.goarch
	}
	return runtime.GOARCH
}
