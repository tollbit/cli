// Package fakecli is the program the self-install tests run in place of the
// CLI binary and of npm. selfinstalltest builds it once per test binary; each
// fake is a copy of that build with its Config appended after the program.
// Loaders on macOS, Linux and Windows ignore bytes past the end of the
// executable image, so every fake is a real native executable and the tests
// need no shell.
package fakecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// RunExitCode is what the fake CLI exits with for any command other than
// `version` and `wait`, so a test can tell that the command ran on it.
const RunExitCode = 7

// marker separates the program from the Config appended to it.
const marker = "\x00selfinstalltest-fakecli-config:"

type (
	// Config is what one fake does.
	Config struct {
		// Version is what the fake CLI prints for `version`.
		Version string `json:"version,omitempty"`
		// NPM, when set, makes the fake an npm instead of a CLI.
		NPM *NPM `json:"npm,omitempty"`
	}

	// NPM is a fake npm for a global install of the CLI.
	NPM struct {
		// Prefix is what `npm prefix -g` prints.
		Prefix string `json:"prefix"`
		// Exe is the CLI binary `npm install -g <pkg>@<version>` replaces
		// with a fake CLI of that version.
		Exe string `json:"exe"`
		// Calls is the file each call's arguments are appended to, one line
		// per call.
		Calls string `json:"calls"`
		// Fail makes `npm install` fail.
		Fail bool `json:"fail"`
	}
)

// Append returns a fake: program, the plain build of this package's command,
// followed by config.
func Append(program []byte, config Config) []byte {
	// Config holds only strings and bools, so Marshal cannot fail.
	data, _ := json.Marshal(config)
	fake := slices.Clip(program)
	fake = append(fake, marker...)
	return append(fake, data...)
}

// Split returns the program and the Config of a fake built by Append.
func Split(fake []byte) ([]byte, Config, error) {
	// The program holds marker as a string constant too, so the appended
	// one is the last.
	i := bytes.LastIndex(fake, []byte(marker))
	if i < 0 {
		return nil, Config{}, errors.New("not a fake: no config")
	}
	var config Config
	if err := json.Unmarshal(fake[i+len(marker):], &config); err != nil {
		return nil, Config{}, fmt.Errorf("not a fake: %w", err)
	}
	return fake[:i], config, nil
}

// Main runs the fake at os.Executable() with args and returns its exit code.
//
// The fake CLI prints its Version for `version`, and for `wait` reads stdin
// until it closes, so a test can replace it while it runs. Any other command
// prints its arguments and every TOLLBIT_ environment variable, then exits
// with RunExitCode.
//
// The fake npm records each call in Calls, prints Prefix for `prefix -g`,
// and for `install -g <pkg>@<version>` replaces Exe with a fake CLI of that
// version, or fails when Fail is set.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	exe, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	fake, err := os.ReadFile(exe)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, config, err := Split(fake)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", exe, err)
		return 1
	}
	if config.NPM != nil {
		return runNPM(*config.NPM, args, stdout, stderr)
	}
	return runCLI(config, args, stdin, stdout)
}

func runCLI(config Config, args []string, stdin io.Reader, stdout io.Writer) int {
	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "version":
		_, _ = fmt.Fprintln(stdout, config.Version)
		return 0
	case "wait":
		_, _ = io.Copy(io.Discard, stdin)
		return 0
	}
	_, _ = fmt.Fprintf(stdout, "args: %s\n", strings.Join(args, " "))
	env := os.Environ()
	slices.Sort(env)
	for _, kv := range env {
		if strings.HasPrefix(kv, "TOLLBIT_") {
			_, _ = fmt.Fprintln(stdout, kv)
		}
	}
	return RunExitCode
}

func runNPM(npm NPM, args []string, stdout, stderr io.Writer) int {
	if err := appendLine(npm.Calls, strings.Join(args, " ")); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 0 {
		return 0
	}
	switch args[0] {
	case "prefix":
		_, _ = fmt.Fprintln(stdout, npm.Prefix)
	case "install":
		if npm.Fail {
			_, _ = fmt.Fprintln(stderr, "npm error code E404")
			return 1
		}
		spec := args[len(args)-1]
		version := spec[strings.LastIndex(spec, "@")+1:]
		if err := installCLI(npm.Exe, version); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, "changed 1 package in 1s")
	}
	return 0
}

// installCLI replaces the fake CLI at exe with one of version.
func installCLI(exe, version string) error {
	current, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	program, _, err := Split(current)
	if err != nil {
		return err
	}
	staged := exe + ".new"
	if err := os.WriteFile(staged, Append(program, Config{Version: version}), 0o755); err != nil {
		return err
	}
	return os.Rename(staged, exe)
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, line); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
