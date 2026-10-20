// Package npm updates a CLI installed with `npm install -g` through npm, so
// npm keeps owning the package: the package's postinstall (npm/install.js)
// downloads the matching binary into the same place, and `npm ls` and the
// package.json stay correct.
package npm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/selfinstall"
)

const (
	// Package is the npm package the CLI ships as.
	Package = "@tollbit/tollbit-cli"

	prefixTimeout = 10 * time.Second
	// installTimeout stays below selfinstall.LockStale so another process
	// never takes the lock over while npm still runs.
	installTimeout = 90 * time.Second
)

var (
	// ErrWindows means npm cannot update the CLI on Windows: it removes the
	// package directory first and cannot delete the running binary.
	ErrWindows = errors.New("npm cannot replace the running binary on Windows")
	// ErrNotFound means npm is not on PATH.
	ErrNotFound = errors.New("npm is not on PATH")
	// ErrNotGlobal means the binary is not the one `npm install -g` manages:
	// it sits in a project's node_modules, came through npx, or was
	// installed by pnpm, Yarn or Volta.
	ErrNotGlobal = errors.New("not a global npm install")
)

// Installer updates a global npm install of the CLI. The zero value of each
// optional field picks the running process's value.
type Installer struct {
	// LockDir holds the lock file that keeps two processes from installing
	// at once.
	LockDir string
	// Executable is the binary npm replaces; os.Executable() when empty.
	// Symlinks are followed.
	Executable string
	// Output receives npm's stdout and stderr; nil discards them.
	Output io.Writer

	goos string
}

// Install runs `npm install -g @tollbit/tollbit-cli@<target>`, unless the
// binary on disk is already at least that new. current is the running
// version.
func (i Installer) Install(ctx context.Context, current, target string) (selfinstall.Result, error) {
	if err := selfinstall.CheckNewer(current, target); err != nil {
		return selfinstall.Result{}, err
	}
	if i.goosOrDefault() == "windows" {
		return selfinstall.Result{}, ErrWindows
	}
	exe, err := selfinstall.Executable(i.Executable)
	if err != nil {
		return selfinstall.Result{}, err
	}
	if manager := packageManager(exe); manager != "" {
		return selfinstall.Result{}, fmt.Errorf("%w: %s", ErrNotGlobal, manager)
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return selfinstall.Result{}, ErrNotFound
	}
	prefix, err := globalPrefix(ctx, npm)
	if err != nil {
		return selfinstall.Result{}, err
	}
	if err := checkGlobal(exe, prefix); err != nil {
		return selfinstall.Result{}, err
	}
	return selfinstall.InstallLocked(ctx, i.LockDir, exe, target, func(ctx context.Context) error {
		return i.run(ctx, npm, target)
	})
}

func (i Installer) run(ctx context.Context, npm, target string) error {
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	out := i.Output
	if out == nil {
		out = io.Discard
	}
	spec := Package + "@" + target
	cmd := exec.CommandContext(ctx, npm, "install", "-g", spec)
	// Stdin stays empty: the command that runs again may read it.
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install -g %s: %w", spec, err)
	}
	return nil
}

func (i Installer) goosOrDefault() string {
	if i.goos != "" {
		return i.goos
	}
	return runtime.GOOS
}

// packageManager names the tool other than npm that put exe where it is, or
// returns "" when the path shows none. Their global folders hold a copy npm
// does not manage, so `npm install -g` would install a second one.
func packageManager(exe string) string {
	for _, part := range strings.Split(filepath.ToSlash(exe), "/") {
		switch part {
		case ".volta":
			return "installed with Volta"
		case ".pnpm", "pnpm":
			return "installed with pnpm"
		case ".yarn", "yarn":
			return "installed with Yarn"
		case "_npx":
			return "run through npx"
		}
	}
	return ""
}

// globalPrefix returns `npm prefix -g`, the folder npm installs global
// packages under.
func globalPrefix(ctx context.Context, npm string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, prefixTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, npm, "prefix", "-g").Output()
	if err != nil {
		return "", fmt.Errorf("npm prefix -g: %w", err)
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return "", errors.New("npm prefix -g printed nothing")
	}
	return prefix, nil
}

// checkGlobal checks that exe is the binary in npm's global copy of the
// package under prefix, and that npm can replace that copy and its bin link
// without elevated permissions. On macOS and Linux npm keeps global packages
// in <prefix>/lib/node_modules and their links in <prefix>/bin.
func checkGlobal(exe, prefix string) error {
	root := filepath.Join(prefix, "lib", "node_modules")
	pkgDir, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(Package)))
	if err != nil {
		return fmt.Errorf("%w: %s is not installed under %s", ErrNotGlobal, Package, root)
	}
	if rel, err := filepath.Rel(pkgDir, exe); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %s is not under %s", ErrNotGlobal, exe, pkgDir)
	}
	for _, dir := range []string{filepath.Dir(pkgDir), filepath.Join(prefix, "bin")} {
		if err := selfinstall.CheckWritable(dir); err != nil {
			return err
		}
	}
	return nil
}
