package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
	"github.com/tollbit/tollbit-cli/internal/version"
)

type (
	ClientConfig struct {
		BaseURL string
	}

	AgentIdentity struct {
		Name      string      `json:"name"`
		UserAgent string      `json:"user_agent,omitempty"`
		WBA       *WebBotAuth `json:"wba,omitempty"`
	}

	AgentTokenOptions struct {
		TTLSeconds *int32
	}

	ConsentRedirectStartRequest struct {
		RedirectURI         string `json:"redirect_uri"`
		State               string `json:"state"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}

	ConsentRedirectStartResponse struct {
		ChallengeID string `json:"challenge_id"`
		ConsentURL  string `json:"consent_url"`
		ExpiresAt   string `json:"expires_at"`
	}

	ConsentRedirectTokenRequest struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"code_verifier"`
		RedirectURI  string `json:"redirect_uri"`
	}

	WebBotAuth struct {
		Dir string `json:"dir"`
		Req bool   `json:"req"`
		Ver int32  `json:"ver"`
	}

	AgentTokenRequest struct {
		AgentIdentifier string      `json:"agent_identifier"`
		TTLSeconds      *int32      `json:"ttl_seconds,omitempty"`
		UA              *string     `json:"ua,omitempty"`
		WBA             *WebBotAuth `json:"wba,omitempty"`
	}

	agentTokenResponse struct {
		Token string `json:"token"`
	}

	Client struct {
		baseURL *url.URL
		http    *http.Client
	}

	requestOption func(*http.Request)
)

func New(cfg ClientConfig) (*Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("auth base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: parsed,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) CreateAgentToken(ctx context.Context, identity AgentIdentity, opts AgentTokenOptions) (agent.Token, error) {
	if strings.TrimSpace(identity.Name) == "" {
		return agent.Token{}, errors.New("agent name is required")
	}

	var ua *string
	if identity.UserAgent != "" {
		ua = &identity.UserAgent
	}

	req := AgentTokenRequest{
		AgentIdentifier: identity.Name,
		TTLSeconds:      opts.TTLSeconds,
		UA:              ua,
		WBA:             identity.WBA,
	}

	u := c.resolve("/agent/v1/tokens/identity")
	var out agentTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, u.String(), req, &out, withUserAgent(identity.UserAgent)); err != nil {
		return agent.Token{}, err
	}
	return agent.Token{RawToken: out.Token}, nil
}

func (c *Client) StartAgentConsentRedirect(ctx context.Context, token agent.Token, req ConsentRedirectStartRequest) (ConsentRedirectStartResponse, error) {
	if strings.TrimSpace(token.RawToken) == "" {
		return ConsentRedirectStartResponse{}, errors.New("agent token is required")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return ConsentRedirectStartResponse{}, errors.New("redirect uri is required")
	}
	if strings.TrimSpace(req.State) == "" {
		return ConsentRedirectStartResponse{}, errors.New("state is required")
	}
	if strings.TrimSpace(req.CodeChallenge) == "" {
		return ConsentRedirectStartResponse{}, errors.New("code challenge is required")
	}
	if strings.TrimSpace(req.CodeChallengeMethod) == "" {
		req.CodeChallengeMethod = "S256"
	}

	u := c.resolve("/agent/v1/consent/redirect/start")
	var out ConsentRedirectStartResponse
	if err := c.doJSON(ctx, http.MethodPost, u.String(), req, &out, withBearerToken(token)); err != nil {
		return ConsentRedirectStartResponse{}, err
	}
	return out, nil
}

func (c *Client) RedeemAgentConsentRedirect(ctx context.Context, token agent.Token, req ConsentRedirectTokenRequest) (agent.Token, error) {
	if strings.TrimSpace(token.RawToken) == "" {
		return agent.Token{}, errors.New("agent token is required")
	}
	if strings.TrimSpace(req.Code) == "" {
		return agent.Token{}, errors.New("authorization code is required")
	}
	if strings.TrimSpace(req.CodeVerifier) == "" {
		return agent.Token{}, errors.New("code verifier is required")
	}
	if strings.TrimSpace(req.RedirectURI) == "" {
		return agent.Token{}, errors.New("redirect uri is required")
	}

	u := c.resolve("/agent/v1/consent/redirect/token")
	var out agentTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, u.String(), req, &out, withBearerToken(token)); err != nil {
		return agent.Token{}, err
	}
	return agent.Token{RawToken: out.Token}, nil
}

func withUserAgent(userAgent string) requestOption {
	return func(req *http.Request) {
		if userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
	}
}

func withBearerToken(token agent.Token) requestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token.RawToken)
	}
}

func (c *Client) doJSON(ctx context.Context, method, rawURL string, body any, out any, opts ...requestOption) error {
	reqBody, err := encodeBody(body)
	if err != nil {
		return err
	}
	var reader io.Reader
	if reqBody != nil {
		reader = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tollbit-Client", version.ClientHeader())
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(req)
	}
	logRequest(ctx, req, reqBody)

	resp, err := c.http.Do(req)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).
			Str("method", method).
			Str("url", rawURL).
			Msg("auth request transport error")
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logResponse(ctx, method, rawURL, reqBody, resp.StatusCode, resp.Status, respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiErrorFromBody(resp.Status, respBody)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func apiErrorFromBody(status string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("request failed: %s", status)
	}
	// TODO: Use internal/errorsx.ParseResponseError once client error handling is consolidated.
	var problem struct {
		Detail *string `json:"detail"`
		Title  string  `json:"title"`
		Status int     `json:"status"`
		Type   string  `json:"type"`
	}
	if err := json.Unmarshal(body, &problem); err == nil && problem.Detail != nil {
		return fmt.Errorf("%s: %s", status, *problem.Detail)
	}
	return fmt.Errorf("request failed: %s: %s", status, strings.TrimSpace(string(body)))
}

func (c *Client) resolve(p string) *url.URL {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + p
	return &u
}

func logRequest(ctx context.Context, req *http.Request, body []byte) {
	e := zerolog.Ctx(ctx).Debug().
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Str("accept", req.Header.Get("Accept")).
		Str("content_type", req.Header.Get("Content-Type"))
	if userAgent := req.Header.Get("User-Agent"); userAgent != "" {
		e = e.Str("user_agent", userAgent)
	}
	if token := req.Header.Get("Authorization"); token != "" {
		e = e.Str("authorization", redactSecret(token))
	}
	if len(body) > 0 {
		e = e.Str("request_body", string(body))
	}
	e.Msg("auth request")
}

func logResponse(ctx context.Context, method, rawURL string, reqBody []byte, statusCode int, status string, respBody []byte) {
	loggedBody := redactLogBody(respBody)
	zerolog.Ctx(ctx).Debug().
		Str("method", method).
		Str("url", rawURL).
		Int("status_code", statusCode).
		Str("status", status).
		Str("response_body", loggedBody).
		Msg("auth response")

	if statusCode < 200 || statusCode >= 300 {
		e := zerolog.Ctx(ctx).Warn().
			Str("method", method).
			Str("url", rawURL).
			Int("status_code", statusCode).
			Str("status", status).
			Str("response_body", loggedBody)
		if len(reqBody) > 0 {
			e = e.Str("request_body", string(reqBody))
		}
		e.Msg("auth response error")
	}
}

func redactLogBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return truncateLog(s)
	}
	if tok, ok := m["token"]; ok {
		raw := strings.TrimSpace(string(tok))
		redacted, err := json.Marshal(redactSecret(strings.Trim(raw, `"`)))
		if err == nil {
			m["token"] = redacted
		}
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return truncateLog(s)
	}
	return string(encoded)
}

func truncateLog(s string) string {
	const max = 2048
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func redactSecret(value string) string {
	if len(value) <= 10 {
		return value
	}
	return value[:6] + "..." + value[len(value)-4:]
}
