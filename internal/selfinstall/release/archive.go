package release

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// maxBinarySize caps how much an archive entry may expand to, so a
	// corrupt or hostile archive cannot fill the disk.
	maxBinarySize = 512 << 20
	// oldSuffix names the binary a Windows install moves aside.
	oldSuffix = ".old"
)

// releaseAsset names the files GoReleaser publishes for one version and
// platform (see .goreleaser.yaml).
type releaseAsset struct {
	archive   string
	checksums string
	binary    string
	zip       bool
}

func (i Installer) asset(version string) releaseAsset {
	goos := i.goosOrDefault()
	base := fmt.Sprintf("%s_%s_%s_%s", binaryName, version, goos, i.goarchOrDefault())
	a := releaseAsset{
		archive:   base + ".tar.gz",
		checksums: fmt.Sprintf("%s_%s_checksums.txt", binaryName, version),
		binary:    binaryName,
	}
	if goos == "windows" {
		a.archive = base + ".zip"
		a.binary = binaryName + ".exe"
		a.zip = true
	}
	return a
}

func (i Installer) client() *http.Client {
	if i.HTTPClient != nil {
		return i.HTTPClient
	}
	return http.DefaultClient
}

func (i Installer) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := i.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// fetchChecksums returns the archive-name-to-SHA-256 map from a GoReleaser
// checksums file ("<sha256>  <name>" per line).
func (i Installer) fetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	body, err := i.get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	sums := make(map[string]string)
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 {
			sums[fields[1]] = fields[0]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	return sums, nil
}

// fetchFile saves url to dest and returns the content's hex SHA-256.
func (i Installer) fetchFile(ctx context.Context, url, dest string) (string, error) {
	body, err := i.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, hash), body); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// extractBinary writes the archive's binary to a new executable file in dir
// and returns its path. Staging it beside the binary it replaces keeps the
// final rename on one filesystem.
func extractBinary(archive string, asset releaseAsset, dir string) (string, error) {
	staged, err := os.CreateTemp(dir, ".tollbit-update-*")
	if err != nil {
		return "", fmt.Errorf("stage new binary: %w", err)
	}
	name := staged.Name()
	if asset.zip {
		err = extractZip(archive, asset.binary, staged)
	} else {
		err = extractTarGz(archive, asset.binary, staged)
	}
	if closeErr := staged.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(name, 0o755)
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func extractTarGz(archive, binary string, dest io.Writer) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("archive did not contain %s", binary)
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
		}
		if hdr.Typeflag == tar.TypeReg && path.Clean(hdr.Name) == binary {
			return copyBinary(dest, tr)
		}
	}
}

func extractZip(archive, binary string, dest io.Writer) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
	}
	defer func() { _ = zr.Close() }()
	for _, entry := range zr.File {
		if entry.Mode().IsRegular() && path.Clean(entry.Name) == binary {
			r, err := entry.Open()
			if err != nil {
				return err
			}
			defer func() { _ = r.Close() }()
			return copyBinary(dest, r)
		}
	}
	return fmt.Errorf("archive did not contain %s", binary)
}

func copyBinary(dest io.Writer, src io.Reader) error {
	n, err := io.Copy(dest, io.LimitReader(src, maxBinarySize+1))
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}
	if n > maxBinarySize {
		return errors.New("extract binary: binary exceeds size limit")
	}
	return nil
}

// replace moves staged over exe. Windows cannot replace a running
// executable, but it can rename one, so there the old binary moves aside to
// exe.old first; RemoveOldBinary deletes it on a later run.
func replace(staged, exe, goos string) error {
	if goos != "windows" {
		if err := os.Rename(staged, exe); err != nil {
			return fmt.Errorf("replace %s: %w", exe, err)
		}
		return nil
	}
	old := exe + oldSuffix
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("move %s aside: %w", exe, err)
	}
	if err := os.Rename(staged, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	return nil
}
