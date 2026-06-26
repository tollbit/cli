package cli

import (
	"errors"
	"fmt"
	"strings"
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
