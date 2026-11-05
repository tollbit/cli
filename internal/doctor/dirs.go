package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/tollbit/cli/internal/configuration"
)

func directoryChecks(opts Options) []Check {
	checks := []Check{
		requiredDirCheck("state", opts.Config.Runtime.StateDir, "runtime.state_dir"),
		requiredDirCheck("credentials", opts.Config.Credentials.StorageDir, "credentials.storage_dir"),
	}
	if exe, err := opts.Executable(); err == nil {
		checks = append(checks, installDirCheck(parentDir(resolvePath(exe))))
	}
	return checks
}

// requiredDirCheck fails when the CLI cannot create dir or write a file in
// it, as every command that saves state or credentials would. configPath is
// the setting that chooses dir, named in the hint.
func requiredDirCheck(name, dir, configPath string) Check {
	c := Check{Group: GroupDirectories, Name: name, Path: dir}
	hint := "fix the permissions on " + dir + ", or set " + configuration.EnvName(configPath) + " to a directory you can write"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		c.Status = StatusFail
		c.Detail = "cannot create " + dir + ": " + describeFSError(err)
		c.Hint = hint
		return c
	}
	if err := probeWrite(dir); err != nil {
		c.Status = StatusFail
		c.Detail = "cannot write to " + dir + ": " + describeFSError(err)
		c.Hint = hint
		return c
	}
	c.Status = StatusPass
	c.Detail = dir + " (writable)"
	return c
}

// installDirCheck only warns: the CLI runs from a read-only directory, but
// cannot update itself there.
func installDirCheck(dir string) Check {
	c := Check{Group: GroupDirectories, Name: "install", Path: dir}
	if err := probeWrite(dir); err != nil {
		c.Status = StatusWarn
		c.Detail = "cannot write to " + dir + ": " + describeFSError(err)
		c.Hint = "the CLI cannot update itself here; reinstall it into a directory you own"
		return c
	}
	c.Status = StatusPass
	c.Detail = dir + " (writable; needed for self-update)"
	return c
}

// probeWrite creates, writes and removes a temp file in dir.
func probeWrite(dir string) error {
	f, err := os.CreateTemp(dir, ".tollbit-doctor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := f.WriteString("ok"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func describeFSError(err error) string {
	var pathErr *fs.PathError
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.As(err, &pathErr):
		return fmt.Sprintf("%s: %v", pathErr.Op, pathErr.Err)
	default:
		return err.Error()
	}
}

// resolvePath follows symlinks so two paths to one file compare equal; it
// returns path unchanged when that fails.
func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func samePath(a, b string) bool {
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return os.SameFile(ai, bi)
}

func parentDir(path string) string {
	return filepath.Dir(path)
}
