// Package client builds the HTTP client that every request the CLI sends goes
// through, so each one identifies the CLI. The API clients live in its
// subpackages. Build HTTP clients with New rather than &http.Client{}.
package client

import (
	"net/http"
	"time"

	"github.com/tollbit/cli/internal/version"
)

const (
	accessMethodHeaderName = "X-Tollbit-Access-Method"
	accessMethod           = "cli"
	clientHeaderName       = "X-Tollbit-Client"
	userAgentHeaderName    = "User-Agent"
)

// identifyingTransport always sets X-Tollbit-Access-Method (backends read it
// first to tell TollBit clients apart) and X-Tollbit-Client (the gateway's
// update check reads it), and sets User-Agent unless the request already set
// its own.
type identifyingTransport struct {
	base http.RoundTripper
}

// New returns an HTTP client that identifies the CLI on every request
// and sends through base (http.DefaultTransport when nil). A zero timeout
// means none. Callers set anything else they need, such as CheckRedirect.
func New(timeout time.Duration, base http.RoundTripper) *http.Client {
	if base == nil {
		base = http.DefaultTransport
	}
	return &http.Client{Timeout: timeout, Transport: identifyingTransport{base: base}}
}

func (t identifyingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A RoundTripper must not modify the caller's request.
	req = req.Clone(req.Context())
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set(accessMethodHeaderName, accessMethod)
	req.Header.Set(clientHeaderName, version.ClientHeader())
	if req.Header.Get(userAgentHeaderName) == "" {
		req.Header.Set(userAgentHeaderName, version.HTTPUserAgent())
	}
	return t.base.RoundTrip(req)
}
