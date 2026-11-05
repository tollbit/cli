package doctor

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tollbit/cli/internal/version"
)

// Error kinds a host check reports, so support and agents can tell a DNS
// failure from a proxy block without reading the detail text.
const (
	KindDNS               = "dns"
	KindConnect           = "connect"
	KindTimeout           = "timeout"
	KindProxyUnreachable  = "proxy_unreachable"
	KindProxyRejected     = "proxy_rejected"
	KindProxyAuthRequired = "proxy_auth_required"
	KindTLSIntercepted    = "tls_intercepted"
	KindTLS               = "tls_other"
	KindOther             = "other"
)

var proxyEnvVars = []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy"}

type target struct {
	host string
	// uses names the config settings that point at host.
	uses []string
	// url is the request doctor makes. For the gateway it is the version
	// policy, which needs no token and only the real origin answers with 200.
	url string
	// followRedirects is set for release downloads, which GitHub redirects to
	// a separate asset host the CLI must also reach.
	followRedirects bool
}

func networkChecks(ctx context.Context, opts Options) []Check {
	checks := proxyChecks(opts)
	targets := hostTargets(opts)
	return append(checks, parallel(targets, func(t target) Check {
		return probeHost(ctx, opts, t)
	})...)
}

func proxyChecks(opts Options) []Check {
	var set []string
	httpsProxySet := false
	for _, name := range proxyEnvVars {
		value := strings.TrimSpace(opts.Getenv(name))
		if value == "" {
			continue
		}
		if strings.EqualFold(name, "HTTPS_PROXY") {
			httpsProxySet = true
		}
		if !strings.EqualFold(name, "NO_PROXY") {
			value = redactProxy(value)
		}
		set = append(set, name+"="+value)
	}
	env := Check{Group: GroupNetwork, Name: "proxy", Status: StatusInfo, Detail: "no proxy environment variables set"}
	if len(set) > 0 {
		env.Detail = strings.Join(set, ", ")
	}
	checks := []Check{env}
	system := opts.SystemProxy()
	if system == "" {
		return checks
	}
	c := Check{Group: GroupNetwork, Name: "system proxy", Status: StatusInfo, Detail: "OS proxy: " + system}
	if !httpsProxySet {
		c.Status = StatusWarn
		c.Detail += "; the CLI does not use OS proxy settings and HTTPS_PROXY is not set"
		c.Hint = "if this network requires the proxy, set HTTPS_PROXY to it (for example HTTPS_PROXY=http://host:port)"
	}
	return append(checks, c)
}

// hostTargets lists one target per host the configuration points at, in the
// order the hosts first appear.
func hostTargets(opts Options) []target {
	cfg := opts.Config
	var targets []target
	add := func(use, rawURL string, followRedirects bool) {
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || u.Host == "" {
			return
		}
		for i := range targets {
			if targets[i].host == u.Host {
				targets[i].uses = append(targets[i].uses, use)
				return
			}
		}
		targets = append(targets, target{host: u.Host, uses: []string{use}, url: u.String(), followRedirects: followRedirects})
	}
	add("auth", cfg.Auth.BaseURL, false)
	// Foundry before gateway: they share a host, and the version policy is
	// the better probe for it.
	add("foundry", strings.TrimRight(cfg.Foundry.BaseURL, "/")+"/cli/v1/version-policy", false)
	add("gateway", cfg.Gateway.BaseURL, false)
	if cfg.Analytics.Enabled {
		add("analytics", cfg.Analytics.BaseURL, false)
	}
	if cfg.Update.CheckEnabled {
		v := version.Version
		add("update", strings.TrimRight(cfg.Update.ReleaseBaseURL, "/")+"/v"+v+"/tollbit_"+v+"_checksums.txt", true)
	}
	return targets
}

func probeHost(ctx context.Context, opts Options, t target) Check {
	c := Check{Group: GroupNetwork, Name: t.host, Host: t.host}
	uses := " (" + strings.Join(t.uses, ", ") + ")"

	ctx, cancel := context.WithTimeout(ctx, opts.HostTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return failed(c, KindOther, "bad URL "+t.url+": "+err.Error(), "")
	}
	req.Header.Set("User-Agent", version.HTTPUserAgent())
	proxyURL, err := opts.Proxy(req)
	if err != nil {
		return failed(c, KindProxyUnreachable, "invalid proxy setting: "+err.Error(), "check HTTPS_PROXY")
	}

	// Through a proxy the proxy resolves the name, so a local DNS failure
	// is expected and only matters without one.
	if _, err := net.DefaultResolver.LookupHost(ctx, req.URL.Hostname()); err != nil && proxyURL == nil {
		return failed(c, KindDNS, "DNS lookup failed: "+err.Error(),
			"check your DNS or VPN, or ask IT whether "+t.host+" is blocked")
	}

	var connectStatus atomic.Int32
	transport := opts.Transport.Clone()
	transport.Proxy = opts.Proxy
	transport.DisableKeepAlives = true
	transport.OnProxyConnectResponse = func(_ context.Context, _ *url.URL, _ *http.Request, resp *http.Response) error {
		connectStatus.Store(int32(resp.StatusCode))
		return nil
	}
	var hops []string
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if !t.followRedirects {
				return http.ErrUseLastResponse
			}
			// Keep http.Client's default limit, so a redirect loop fails
			// fast instead of reading as a timeout.
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if next.URL.Host != via[len(via)-1].URL.Host {
				hops = append(hops, next.URL.Host)
			}
			return nil
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		kind, detail, hint := classify(err, int(connectStatus.Load()), proxyURL, t.host, opts.HostTimeout)
		return failed(c, kind, detail+uses, hint)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	host := t.host
	if len(hops) > 0 {
		host = hops[len(hops)-1]
	}
	detail := fmt.Sprintf("HTTP %d in %s", resp.StatusCode, elapsed)
	if len(hops) > 0 {
		detail += " via " + strings.Join(hops, " -> ")
	}
	if proxyURL != nil {
		detail += ", through proxy " + redactProxy(proxyURL.String())
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		detail += ", certificate issued by " + certIssuer(resp.TLS.PeerCertificates[0])
	}

	switch {
	case resp.StatusCode == http.StatusProxyAuthRequired:
		return failed(c, KindProxyAuthRequired, detail+uses, "the proxy requires authentication; ask IT how to authenticate command-line tools")
	// No TollBit or GitHub endpoint doctor calls answers 403, so a 403 comes
	// from something in between: a proxy, firewall or security product.
	case resp.StatusCode == http.StatusForbidden:
		return failed(c, "http_403", detail+uses, "a proxy or firewall blocked "+host+"; ask IT to allow it")
	case resp.StatusCode >= 500:
		return failed(c, fmt.Sprintf("http_%d", resp.StatusCode), detail+uses, host+" or a proxy in between returned a server error; try again, then contact TollBit support")
	}
	c.Status = StatusPass
	c.Detail = detail + uses
	return c
}

// classify turns a failed request into an error kind, a detail and a hint.
// connectStatus is the proxy's answer to CONNECT, or 0 when there was none.
func classify(err error, connectStatus int, proxyURL *url.URL, host string, timeout time.Duration) (kind, detail, hint string) {
	proxy := ""
	if proxyURL != nil {
		proxy = redactProxy(proxyURL.String())
	}
	var (
		certErr    *tls.CertificateVerificationError
		unknownCA  x509.UnknownAuthorityError
		recordErr  tls.RecordHeaderError
		dnsErr     *net.DNSError
		opErr      *net.OpError
		netErr     net.Error
		hostErr    x509.HostnameError
		invalidErr x509.CertificateInvalidError
	)
	switch {
	case connectStatus == http.StatusProxyAuthRequired:
		return KindProxyAuthRequired, "proxy " + proxy + " requires authentication (CONNECT 407)",
			"ask IT how to authenticate command-line tools to the proxy"
	case connectStatus != 0 && connectStatus != http.StatusOK:
		return KindProxyRejected, fmt.Sprintf("proxy %s refused to connect to %s (CONNECT %d)", proxy, host, connectStatus),
			"ask IT to allow " + host + " through the proxy"
	case errors.As(err, &opErr) && opErr.Op == "proxyconnect":
		return KindProxyUnreachable, "cannot reach proxy " + proxy + ": " + rootCause(opErr.Err),
			"check HTTPS_PROXY; the proxy address may be wrong or down"
	case errors.As(err, &certErr) && errors.As(err, &unknownCA):
		issuer := "an unknown issuer"
		if len(certErr.UnverifiedCertificates) > 0 {
			issuer = `"` + certIssuer(certErr.UnverifiedCertificates[0]) + `"`
		}
		return KindTLSIntercepted, "TLS certificate for " + host + " is issued by " + issuer + ", which this machine does not trust",
			"a proxy or security product is inspecting TLS; ask IT to exempt " + host + " from TLS inspection, or to install its root certificate in the system store"
	case errors.As(err, &hostErr):
		return KindTLSIntercepted, "TLS certificate is not for " + host + ": " + hostErr.Error(),
			"something in between answered for " + host + "; ask IT whether it is blocked or inspected"
	case errors.As(err, &invalidErr), errors.As(err, &recordErr), errors.As(err, &certErr):
		return KindTLS, "TLS failed: " + rootCause(err), "ask IT whether " + host + " is blocked or inspected"
	case errors.As(err, &dnsErr):
		if proxyURL != nil {
			return KindProxyUnreachable, "proxy " + proxy + " does not resolve: " + dnsErr.Error(), "check HTTPS_PROXY"
		}
		return KindDNS, "DNS lookup failed: " + dnsErr.Error(), "check your DNS or VPN, or ask IT whether " + host + " is blocked"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return KindTimeout, fmt.Sprintf("no answer within %s", timeout),
			"a firewall may be dropping traffic to " + host + ", or this network needs a proxy (set HTTPS_PROXY)"
	case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return KindConnect, "connection refused or reset: " + rootCause(err),
			"a firewall may be blocking " + host + " on port 443"
	default:
		return KindOther, rootCause(err), ""
	}
}

func failed(c Check, kind, detail, hint string) Check {
	c.Status = StatusFail
	c.ErrorKind = kind
	c.Detail = detail
	c.Hint = hint
	return c
}

// rootCause drops the *url.Error wrapper, which repeats the method and URL.
func rootCause(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

func certIssuer(cert *x509.Certificate) string {
	if cert.Issuer.CommonName != "" {
		return cert.Issuer.CommonName
	}
	if len(cert.Issuer.Organization) > 0 {
		return cert.Issuer.Organization[0]
	}
	return cert.Issuer.String()
}

// redactProxy returns a proxy URL without the user name or password it may
// hold.
func redactProxy(raw string) string {
	candidate := raw
	if !strings.Contains(candidate, "://") {
		candidate = "http://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil {
		if i := strings.LastIndex(raw, "@"); i >= 0 {
			return "***@" + raw[i+1:]
		}
		return raw
	}
	if u.User == nil {
		return raw
	}
	u.User = nil
	return strings.Replace(u.String(), "://", "://***@", 1)
}
