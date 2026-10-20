// Package versionpolicy reads the CLI version policy (minimum supported and
// latest released version) from Foundry. The route takes no auth so the CLI
// can check before it has, or needs, an agent token.
package versionpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/errorsx"
	"github.com/tollbit/cli/internal/version"
)

const (
	policyPath = "/cli/v1/version-policy"

	// requestTimeout is short because the check runs before every command and
	// must never hold one up for long.
	requestTimeout = 5 * time.Second
)

type (
	Config struct {
		// BaseURL is foundry.base_url, which already ends in /foundry.
		BaseURL string
	}

	Client interface {
		Get(ctx context.Context) (Policy, error)
	}

	// Policy is the server's view of supported CLI versions. Either field can
	// be empty, which means that bound is not enforced.
	Policy struct {
		Minimum string `json:"minimum"`
		Latest  string `json:"latest"`
	}

	client struct {
		baseURL *url.URL
		http    *http.Client
	}
)

var _ Client = (*client)(nil)

func NewClient(cfg Config) (Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("foundry base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &client{
		baseURL: parsed,
		http:    &http.Client{Timeout: requestTimeout},
	}, nil
}

func (c *client) Get(ctx context.Context) (Policy, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + policyPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Policy{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tollbit-Client", version.ClientHeader())
	req.Header.Set("User-Agent", version.HTTPUserAgent())
	zerolog.Ctx(ctx).Debug().
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Msg("version policy request")

	resp, err := c.http.Do(req)
	if err != nil {
		return Policy{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Policy{}, err
	}
	zerolog.Ctx(ctx).Debug().
		Int("status_code", resp.StatusCode).
		Msg("version policy response")
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Policy{}, errorsx.ParseResponseError(ctx, resp.Status, resp.StatusCode, resp.Header, body)
	}
	var policy Policy
	if err := json.Unmarshal(body, &policy); err != nil {
		return Policy{}, err
	}
	policy.Minimum = strings.TrimSpace(policy.Minimum)
	policy.Latest = strings.TrimSpace(policy.Latest)
	return policy, nil
}
