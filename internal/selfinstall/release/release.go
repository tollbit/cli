// Package release replaces the CLI binary with a published release. It
// follows the same steps as scripts/install.sh: download the release archive
// and its checksums, verify the archive's SHA-256, extract the binary, and
// move it into place.
package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/selfinstall"
)

const (
	// DownloadTimeout bounds downloading the archive and its checksums.
	DownloadTimeout = 60 * time.Second

	binaryName = "tollbit"
)

// Installer replaces a CLI binary with a release from the first of BaseURLs
// that serves it. The zero value of each optional field picks the running
// process's value.
type Installer struct {
	// BaseURLs each hold one directory per release tag (v<version>) with the
	// archives and checksums GoReleaser publishes. They are tried in order:
	// a failed download moves on to the next, a checksum mismatch does not.
	BaseURLs []string
	// LockDir holds the lock file that keeps two processes from installing
	// at once.
	LockDir string
	// Executable is the binary to replace; os.Executable() when empty.
	// Symlinks are followed so the real file is replaced.
	Executable string
	// HTTPClient downloads the release; client.New(0, nil) when nil.
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
	if len(i.BaseURLs) == 0 {
		return errors.New("no release base URLs configured")
	}
	tmpDir, err := os.MkdirTemp("", "tollbit-update-*")
	if err != nil {
		return fmt.Errorf("create download directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	asset := i.asset(target)
	archive := filepath.Join(tmpDir, asset.archive)
	var checksums map[string]string
	var got string
	var failures []error
	for _, baseURL := range i.BaseURLs {
		releaseURL := strings.TrimRight(baseURL, "/") + "/v" + target + "/"
		checksums, got, err = i.fetch(ctx, releaseURL, asset, archive)
		if err == nil {
			break
		}
		zerolog.Ctx(ctx).Debug().Err(err).Str("base_url", baseURL).Msg("release download failed")
		failures = append(failures, err)
	}
	if len(failures) == len(i.BaseURLs) {
		return errors.Join(failures...)
	}

	// Whichever source served them, the archive must match its checksums.
	// A mismatch never falls back to the next source.
	want, ok := checksums[asset.archive]
	if !ok {
		return fmt.Errorf("checksum entry not found for %s", asset.archive)
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

// fetch downloads the checksums and the archive from releaseURL, saving the
// archive to dest. It returns the checksums and the archive's SHA-256. Each
// call gets its own DownloadTimeout, so a source that hangs still leaves
// time for the next.
func (i Installer) fetch(ctx context.Context, releaseURL string, asset releaseAsset, dest string) (map[string]string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	checksums, err := i.fetchChecksums(ctx, releaseURL+asset.checksums)
	if err != nil {
		return nil, "", err
	}
	got, err := i.fetchFile(ctx, releaseURL+asset.archive, dest)
	if err != nil {
		return nil, "", err
	}
	return checksums, got, nil
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
