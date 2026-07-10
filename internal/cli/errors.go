package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/installmethod"
)

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	return e.Err.Error()
}

func (e ExitError) Unwrap() error {
	return e.Err
}

func UsageError(format string, args ...any) error {
	return ExitError{Code: 2, Err: fmt.Errorf(format, args...)}
}

func RuntimeError(err error) error {
	if err == nil {
		return nil
	}
	return ExitError{Code: 1, Err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	return 1
}

// RenderError returns the user-facing message for a command error. Backend
// "CLI update required" rejections are rewritten into an update instruction
// matching this binary's install method; everything else renders as-is.
func RenderError(err error) string {
	var problem problemjson.Problem
	if errors.As(err, &problem) && problem.IsCLIUpdateRequired() {
		minimum, _ := problem.StringProperty("minimumVersion")
		latest, _ := problem.StringProperty("latestVersion")
		return installmethod.RequiredInstructions(installmethod.Detect(), minimum, latest)
	}
	return err.Error()
}
