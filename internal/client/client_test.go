package client

import (
	"net/http"
	"testing"

	"github.com/tollbit/cli/internal/version"
)

type recordingTransport struct {
	got *http.Request
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.got = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

func TestClientSetsIdentityHeaders(t *testing.T) {
	base := &recordingTransport{}
	req, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := New(0, base).Do(req); err != nil {
		t.Fatal(err)
	}

	if got := base.got.Header.Get("User-Agent"); got != version.HTTPUserAgent() {
		t.Errorf("User-Agent = %q, want %q", got, version.HTTPUserAgent())
	}
	if got := base.got.Header.Get("X-Tollbit-Client"); got != version.ClientHeader() {
		t.Errorf("X-Tollbit-Client = %q, want %q", got, version.ClientHeader())
	}
	if got := base.got.Header.Get("X-Tollbit-Access-Method"); got != "cli" {
		t.Errorf("X-Tollbit-Access-Method = %q, want %q", got, "cli")
	}
	if req.Header.Get("X-Tollbit-Client") != "" {
		t.Error("client modified the caller's request")
	}
}

func TestClientKeepsUserAgentTheRequestSet(t *testing.T) {
	base := &recordingTransport{}
	req, err := http.NewRequest(http.MethodGet, "https://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "MyAgent/1.0")

	if _, err := New(0, base).Do(req); err != nil {
		t.Fatal(err)
	}

	if got := base.got.Header.Get("User-Agent"); got != "MyAgent/1.0" {
		t.Errorf("User-Agent = %q, want the request's own", got)
	}
	if got := base.got.Header.Get("X-Tollbit-Client"); got != version.ClientHeader() {
		t.Errorf("X-Tollbit-Client = %q, want %q", got, version.ClientHeader())
	}
	if got := base.got.Header.Get("X-Tollbit-Access-Method"); got != "cli" {
		t.Errorf("X-Tollbit-Access-Method = %q, want %q", got, "cli")
	}
}
