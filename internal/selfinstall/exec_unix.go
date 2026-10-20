//go:build !windows

package selfinstall

import (
	"fmt"
	"os"
	"syscall"
)

// Restart runs the command again on the binary at path, with the same
// arguments and GuardEnvVar set to version. It replaces this process, which
// keeps its PID, stdio and exit code, so it returns only on failure.
func Restart(path, version string) (int, error) {
	env := append(os.Environ(), GuardEnvVar+"="+version)
	if err := syscall.Exec(path, os.Args, env); err != nil {
		return 1, fmt.Errorf("run %s: %w", path, err)
	}
	return 0, nil
}
