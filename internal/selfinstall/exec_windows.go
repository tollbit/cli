//go:build windows

package selfinstall

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Restart runs the command again on the binary at path, with the same
// arguments, this process's stdio and GuardEnvVar set to version. Windows has
// no exec, so it waits for the child and returns the child's exit code.
func Restart(path, version string) (int, error) {
	cmd := exec.Command(path, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), GuardEnvVar+"="+version)
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 1, fmt.Errorf("run %s: %w", path, err)
	}
	return 0, nil
}
