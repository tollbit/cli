package versionpolicy

import (
	"net/http"
	"strings"
)

// latestVersionHeader is set by the gateway on successful responses when this
// CLI version is outdated but still supported.
const latestVersionHeader = "X-Tollbit-Cli-Latest-Version"

// Transport watches responses for the server's latest-version header, so the
// API clients that send requests through it never have to.
type Transport struct {
	// Base sends the request; http.DefaultTransport when nil.
	Base http.RoundTripper
	// HandleVersionUpgradeNotice receives the newest released CLI version
	// whenever a successful response says this CLI is outdated.
	HandleVersionUpgradeNotice func(latest string)
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	// Error responses carry their own version guidance (426
	// cli_update_required), so only successful ones report here.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || t.HandleVersionUpgradeNotice == nil {
		return resp, nil
	}
	if latest := strings.TrimSpace(resp.Header.Get(latestVersionHeader)); latest != "" {
		t.HandleVersionUpgradeNotice(latest)
	}
	return resp, nil
}
