package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/tollbit/cli/internal/client/versionpolicy"
	"github.com/tollbit/cli/internal/cliupdate"
	"github.com/tollbit/cli/internal/configuration"
)

// Process holds what the whole run shares: the update handler and what is
// built from it. main builds one per run, before any command's flags are
// parsed, and every App built during the run holds a reference to it. So only
// add things here that no command flag can change.
type Process struct {
	config configuration.Config

	updateHandler func() *cliupdate.Handler
	versionPolicy func() (versionpolicy.Client, error)
	updateChecker func() (cliupdate.Checker, error)
}

// NewProcess returns the run's Process; update notices go to stderr.
func NewProcess(config configuration.Config, stderr io.Writer) *Process {
	p := &Process{config: config}
	p.updateHandler = sync.OnceValue(func() *cliupdate.Handler {
		h := cliupdate.NewHandler(stderr)
		h.ConfigureSelfInstall(config)
		return h
	})
	p.versionPolicy = sync.OnceValues(p.buildVersionPolicy)
	p.updateChecker = sync.OnceValues(p.buildUpdateChecker)
	return p
}

// UpdateHandler returns the run's one update handler, so the update notice
// prints at most once however many Apps report to it.
func (p *Process) UpdateHandler() *cliupdate.Handler {
	return p.updateHandler()
}

// UpdateChecker returns the check that compares this CLI with the cached
// version policy, refreshing it once per update.check_interval.
func (p *Process) UpdateChecker() (cliupdate.Checker, error) {
	return p.updateChecker()
}

// Transport is the HTTP transport for TollBit API clients: it reports the
// gateway's latest-version header to the update handler.
func (p *Process) Transport() http.RoundTripper {
	return versionpolicy.Transport{HandleVersionUpgradeNotice: p.UpdateHandler().HandleVersionUpgradeNotice}
}

// RecordUpdateRequired passes a command's error to the update check, which
// saves the versions from a gateway's 426 cli_update_required response so the
// next command updates. It does nothing when the check is off.
func (p *Process) RecordUpdateRequired(ctx context.Context, err error) {
	if !p.config.Update.CheckEnabled {
		return
	}
	checker, buildErr := p.UpdateChecker()
	if buildErr != nil {
		return
	}
	checker.RecordUpdateRequired(ctx, err)
}

func (p *Process) buildVersionPolicy() (versionpolicy.Client, error) {
	client, err := versionpolicy.NewClient(versionpolicy.Config{BaseURL: p.config.Foundry.BaseURL})
	if err != nil {
		return nil, fmt.Errorf("build version policy client: %w", err)
	}
	return client, nil
}

func (p *Process) buildUpdateChecker() (cliupdate.Checker, error) {
	policy, err := p.versionPolicy()
	if err != nil {
		return cliupdate.Checker{}, err
	}
	return cliupdate.NewChecker(p.config.Runtime.StateDir, policy, p.config.Update.CheckInterval, p.UpdateHandler()), nil
}
