package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tollbit/cli/internal/errorsx/problemjson"
	"github.com/tollbit/cli/internal/installmethod"
)

// ExitCodeAuthorizationPending signals that a detached authorization flow was
// started or is still awaiting end-user browser consent.
const ExitCodeAuthorizationPending = 3

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
// matching this binary's install method, and "entitlement required" rejections
// gain instructions for obtaining the entitlement; everything else renders
// as-is.
func RenderError(err error) string {
	var problem problemjson.Problem
	if errors.As(err, &problem) {
		switch {
		case problem.IsCLIUpdateRequired():
			minimum, _ := problem.StringProperty("minimumVersion")
			latest, _ := problem.StringProperty("latestVersion")
			return installmethod.RequiredInstructions(installmethod.Detect(), minimum, latest)
		case problem.IsEntitlementRequired():
			return renderEntitlementRequired(err, problem)
		}
	}
	return err.Error()
}

// renderEntitlementRequired keeps the original error line and adds how to get
// the entitlement: the server's terms_url when accepting terms grants it,
// otherwise the entitlement name and a pointer to TollBit.
func renderEntitlementRequired(err error, problem problemjson.Problem) string {
	var b strings.Builder
	b.WriteString(err.Error())
	b.WriteString("\n")
	if termsURL, _ := problem.StringProperty("terms_url"); termsURL != "" {
		b.WriteString("Relay ONLY this URL to the end user. The end user must open it in their browser, sign in, and accept the terms:\n")
		b.WriteString(termsURL)
		b.WriteString("\n\nAfter the end user accepts the terms, re-run the command.")
		return b.String()
	}
	if entitlement, _ := problem.StringProperty("entitlement"); entitlement != "" {
		fmt.Fprintf(&b, "Missing capability: %s\n", entitlement)
	}
	b.WriteString("Contact TollBit to enable it for your organization.")
	return b.String()
}
