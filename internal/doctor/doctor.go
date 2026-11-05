// Package doctor checks whether the CLI can run on this machine: that the
// binary is where the shell finds it, that the CLI can write every directory
// it uses, and that every host it calls answers through this network. It
// never reads or prints token values; the auth check reports only whether a
// token is stored and when it expires.
package doctor

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/tollbit/cli/internal/configuration"
	"github.com/tollbit/cli/internal/installmethod"
	"github.com/tollbit/cli/internal/version"
)

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
	StatusInfo Status = "info"
)

const (
	GroupExecution   = "execution"
	GroupDirectories = "directories"
	GroupNetwork     = "network"
	GroupAuth        = "auth"
	GroupSkills      = "skills"
)

// defaultHostTimeout bounds each host probe: long enough for a slow proxy,
// short enough that a blackholed host does not stall the report.
const defaultHostTimeout = 10 * time.Second

type (
	// Check is one line of the report. Path is set for directory checks,
	// Host and ErrorKind for host checks, Version for skill checks.
	Check struct {
		Group     string `json:"group"`
		Name      string `json:"name"`
		Status    Status `json:"status"`
		Detail    string `json:"detail"`
		Hint      string `json:"hint,omitempty"`
		Path      string `json:"path,omitempty"`
		Host      string `json:"host,omitempty"`
		ErrorKind string `json:"error_kind,omitempty"`
		Version   string `json:"version,omitempty"`
	}

	Report struct {
		OK     bool    `json:"ok"`
		Checks []Check `json:"checks"`
	}

	// Options holds the configuration doctor checks and the seams tests use
	// to stand in for the machine. Zero values use the real machine.
	Options struct {
		Config configuration.Config

		Executable    func() (string, error)
		LookPath      func(string) (string, error)
		Getenv        func(string) string
		InstallMethod func() installmethod.Method
		// Proxy picks the proxy for a request, as http.Transport.Proxy does.
		Proxy func(*http.Request) (*url.URL, error)
		// Transport is cloned for each probe; nil uses http.DefaultTransport.
		Transport *http.Transport
		// SystemProxy returns the OS proxy setting the CLI does not use, or "".
		SystemProxy func() string
		// Credentials is read for the auth check; nil skips it.
		Credentials CredentialStore
		// Skills are the agent skills bundled with this CLI.
		Skills      []Skill
		HostTimeout time.Duration
	}
)

// Run runs every check and returns the report. It does not fail: problems
// are checks with StatusFail.
func Run(ctx context.Context, opts Options) Report {
	opts = opts.withDefaults()
	var checks []Check
	checks = append(checks, executionChecks(opts)...)
	checks = append(checks, directoryChecks(opts)...)
	checks = append(checks, networkChecks(ctx, opts)...)
	if opts.Credentials != nil {
		checks = append(checks, authCheck(ctx, opts.Credentials))
	}
	checks = append(checks, skillChecks(opts.Skills)...)
	report := Report{OK: true, Checks: checks}
	for _, c := range checks {
		if c.Status == StatusFail {
			report.OK = false
		}
	}
	return report
}

func (o Options) withDefaults() Options {
	if o.Executable == nil {
		o.Executable = os.Executable
	}
	if o.LookPath == nil {
		o.LookPath = exec.LookPath
	}
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.InstallMethod == nil {
		o.InstallMethod = installmethod.Detect
	}
	if o.Proxy == nil {
		o.Proxy = http.ProxyFromEnvironment
	}
	if o.Transport == nil {
		o.Transport = http.DefaultTransport.(*http.Transport)
	}
	if o.SystemProxy == nil {
		o.SystemProxy = systemProxy
	}
	if o.HostTimeout <= 0 {
		o.HostTimeout = defaultHostTimeout
	}
	return o
}

func executionChecks(opts Options) []Check {
	method := opts.InstallMethod()
	checks := []Check{{
		Group:  GroupExecution,
		Name:   "version",
		Status: StatusInfo,
		Detail: version.String() + " (" + runtime.GOOS + "/" + runtime.GOARCH + ", install method: " + string(method) + ")",
	}}
	exe, err := opts.Executable()
	if err != nil {
		return append(checks, Check{
			Group:  GroupExecution,
			Name:   "binary",
			Status: StatusWarn,
			Detail: "could not find this binary's path: " + err.Error(),
		})
	}
	exe = resolvePath(exe)
	checks = append(checks, Check{Group: GroupExecution, Name: "binary", Status: StatusPass, Detail: exe, Path: exe})
	return append(checks, pathCheck(opts, exe, method))
}

// pathCheck reports whether the shell finds this binary as `tollbit`. An npm
// install puts a wrapper script on PATH rather than the binary itself, so for
// npm any `tollbit` on PATH counts.
func pathCheck(opts Options, exe string, method installmethod.Method) Check {
	c := Check{Group: GroupExecution, Name: "on PATH"}
	found, err := opts.LookPath(opts.Config.App.Name)
	if err != nil {
		c.Status = StatusFail
		c.Detail = "`" + opts.Config.App.Name + "` is not on PATH"
		c.Hint = "add " + parentDir(exe) + " to PATH so shells and agents can run `" + opts.Config.App.Name + "`"
		return c
	}
	found = resolvePath(found)
	c.Path = found
	switch {
	case samePath(found, exe):
		c.Status = StatusPass
		c.Detail = found
	case method == installmethod.MethodNPM:
		c.Status = StatusPass
		c.Detail = found + " (npm wrapper)"
	default:
		c.Status = StatusWarn
		c.Detail = "`" + opts.Config.App.Name + "` on PATH is " + found + ", not this binary"
		c.Hint = "remove the other copy or put " + parentDir(exe) + " first on PATH"
	}
	return c
}

// parallel runs fn for each item at once and returns the results in order.
func parallel[T, R any](items []T, fn func(T) R) []R {
	out := make([]R, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = fn(item)
		}()
	}
	wg.Wait()
	return out
}
