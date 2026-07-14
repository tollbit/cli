package tollbit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tollbit/tollbit-cli/internal/errorsx"
	"github.com/tollbit/tollbit-cli/internal/tokens/agent"
	"github.com/tollbit/tollbit-cli/internal/version"
)

type (
	Config struct {
		BaseURL string
	}

	Client interface {
		Search(ctx context.Context, params SearchParams, token agent.Token) (PagedSearchResultResponse, error)
		BatchGetRates(ctx context.Context, urls []string, token agent.Token) ([]BatchRateResponseV2, error)
		CreateContentAccessToken(ctx context.Context, req CreateContentAccessTokenRequest, token agent.Token) (CreateContentAccessTokenResponse, error)
		GetContent(ctx context.Context, articleURL, contentToken, userAgent string, token agent.Token) (GetContentResponse, error)
	}

	client struct {
		baseURL *url.URL
		http    *http.Client
	}

	SearchParams struct {
		Query               string
		Size                int
		NextToken           string
		Properties          string
		ProgrammaticOnly    bool
		ProgrammaticOnlySet bool
	}

	PagedSearchResultResponse struct {
		NextToken string         `json:"nextToken"`
		Items     []SearchResult `json:"items"`
	}

	SearchResult struct {
		Title         string       `json:"title"`
		URL           string       `json:"url"`
		PublishedDate string       `json:"publishedDate"`
		Publisher     Publisher    `json:"publisher"`
		Availability  Availability `json:"availability"`
	}

	Publisher struct {
		Domain string `json:"domain"`
		Name   string `json:"name"`
	}

	Availability struct {
		Discoverable   bool `json:"discoverable"`
		ReadyToLicense bool `json:"readyToLicense"`
	}

	BatchGetRateRequest struct {
		URLs []string `json:"urls"`
	}

	BatchRateResponseV2 struct {
		URL   string                       `json:"url"`
		Rates []BatchDeveloperRateResponse `json:"rates"`
	}

	BatchDeveloperRateResponse struct {
		Price   RatePriceResponse        `json:"price"`
		License BatchRateLicenseResponse `json:"license"`
		Error   string                   `json:"error"`
	}

	BatchRateLicenseResponse struct {
		Cuid        string                  `json:"cuid"`
		LicenseType string                  `json:"licenseType"`
		LicensePath string                  `json:"licensePath"`
		Permissions []RateLicensePermission `json:"permissions"`
		ValidUntil  string                  `json:"validUntil"`
	}

	RatePriceResponse struct {
		PriceMicros int64  `json:"priceMicros"`
		Currency    string `json:"currency"`
	}

	RateLicensePermission struct {
		Name string `json:"name"`
	}

	CreateContentAccessTokenRequest struct {
		URL            string `json:"url"`
		UserAgent      string `json:"userAgent,omitempty"`
		MaxPriceMicros int64  `json:"maxPriceMicros"`
		Currency       string `json:"currency"`
		LicenseType    string `json:"licenseType"`
		LicenseCuid    string `json:"licenseCuid"`
		Format         string `json:"format,omitempty"`
	}

	CreateContentAccessTokenResponse struct {
		Token string `json:"token"`
	}

	GetContentResponse struct {
		Content  PageContent     `json:"content"`
		Metadata ContentMetadata `json:"metadata"`
		Rate     *ContentRate    `json:"rate,omitempty"`
	}

	PageContent struct {
		Header string `json:"header"`
		Body   string `json:"body"`
		Footer string `json:"footer"`
	}

	ContentMetadata struct {
		Title       string `json:"title,omitempty"`
		Description string `json:"description,omitempty"`
		ImageURL    string `json:"imageUrl,omitempty"`
		Author      string `json:"author,omitempty"`
		Published   string `json:"published,omitempty"`
		Modified    string `json:"modified,omitempty"`
	}

	ContentRate struct {
		Price   RatePriceResponse        `json:"price"`
		License BatchRateLicenseResponse `json:"license"`
	}

	requestOption func(*http.Request)
)

var _ Client = (*client)(nil)

func NewClient(cfg Config) (Client, error) {
	cfg.Normalize()
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("gateway base URL is required")
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	return &client{
		baseURL: parsed,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Config) Normalize() {
	c.BaseURL = strings.TrimSpace(c.BaseURL)
}

func (c *client) Search(ctx context.Context, params SearchParams, token agent.Token) (PagedSearchResultResponse, error) {
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return PagedSearchResultResponse{}, errors.New("search query is required")
	}
	if err := requireAgentToken(token); err != nil {
		return PagedSearchResultResponse{}, err
	}

	q := url.Values{"q": {query}}
	if params.Size > 0 {
		q.Set("size", strconv.Itoa(params.Size))
	}
	if nextToken := strings.TrimSpace(params.NextToken); nextToken != "" {
		q.Set("next-token", nextToken)
	}
	if properties := strings.TrimSpace(params.Properties); properties != "" {
		q.Set("properties", properties)
	}
	if params.ProgrammaticOnlySet {
		q.Set("allowedOnly", strconv.FormatBool(params.ProgrammaticOnly))
	}

	u := c.resolve("/agents/v1/search")
	u.RawQuery = q.Encode()
	var out PagedSearchResultResponse
	return out, c.doJSON(ctx, http.MethodGet, u.String(), nil, &out, withBearerToken(token.RawToken))
}

func (c *client) BatchGetRates(ctx context.Context, urls []string, token agent.Token) ([]BatchRateResponseV2, error) {
	if len(urls) == 0 {
		return nil, errors.New("at least one URL is required")
	}
	if err := requireAgentToken(token); err != nil {
		return nil, err
	}

	u := c.resolve("/agents/v1/rates/batch")
	var out []BatchRateResponseV2
	return out, c.doJSON(ctx, http.MethodPost, u.String(), BatchGetRateRequest{URLs: urls}, &out,
		withBearerToken(token.RawToken),
	)
}

func (c *client) CreateContentAccessToken(ctx context.Context, req CreateContentAccessTokenRequest, token agent.Token) (CreateContentAccessTokenResponse, error) {
	if err := requireAgentToken(token); err != nil {
		return CreateContentAccessTokenResponse{}, err
	}
	if strings.TrimSpace(req.URL) == "" {
		return CreateContentAccessTokenResponse{}, errors.New("content URL is required")
	}

	u := c.resolve("/agents/v1/tokens/content")
	var out CreateContentAccessTokenResponse
	return out, c.doJSON(ctx, http.MethodPost, u.String(), req, &out,
		withBearerToken(token.RawToken),
	)
}

func (c *client) GetContent(ctx context.Context, articleURL, contentToken, userAgent string, token agent.Token) (GetContentResponse, error) {
	if err := requireAgentToken(token); err != nil {
		return GetContentResponse{}, err
	}
	contentToken = strings.TrimSpace(contentToken)
	if contentToken == "" {
		return GetContentResponse{}, errors.New("content token is required")
	}
	resourcePath, err := contentResourcePath(articleURL)
	if err != nil {
		return GetContentResponse{}, err
	}

	u := c.resolve("/agents/v1/content/" + escapeContentResourcePath(resourcePath))
	if parsed, err := url.Parse(strings.TrimSpace(articleURL)); err == nil && parsed.RawQuery != "" {
		u.RawQuery = parsed.RawQuery
	}
	var out GetContentResponse
	return out, c.doJSON(ctx, http.MethodGet, u.String(), nil, &out,
		withBearerToken(token.RawToken),
		withTollbitToken(contentToken),
		withTollbitUserAgent(userAgent),
	)
}

func contentResourcePath(articleURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(articleURL))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return "", errors.New("article URL must include scheme and host")
	}
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return parsed.Host + path, nil
}

func escapeContentResourcePath(resource string) string {
	resource = strings.TrimSpace(resource)
	for strings.HasSuffix(resource, "/") {
		resource = strings.TrimSuffix(resource, "/")
	}
	segments := strings.Split(resource, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func requireAgentToken(token agent.Token) error {
	if strings.TrimSpace(token.RawToken) == "" {
		return errors.New("agent token is required")
	}
	return token.Validate()
}

func withBearerToken(token string) requestOption {
	return func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func withTollbitUserAgent(userAgent string) requestOption {
	return func(req *http.Request) {
		if ua := strings.TrimSpace(userAgent); ua != "" {
			req.Header.Set("Tollbit-User-Agent", ua)
		}
	}
}

func withTollbitToken(token string) requestOption {
	return func(req *http.Request) {
		req.Header.Set("Tollbit-Token", token)
	}
}

func (c *client) doJSON(ctx context.Context, method, rawURL string, body any, out any, opts ...requestOption) error {
	bodyBytes, err := encodeBody(body)
	if err != nil {
		return err
	}
	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tollbit-Client", version.ClientHeader())
	req.Header.Set("User-Agent", version.HTTPUserAgent())
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(req)
	}
	logRequest(ctx, req, bodyBytes)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logResponse(ctx, resp)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errorsx.ParseResponseError(ctx, resp.Status, resp.StatusCode, resp.Header, respBody)
	}
	notifyUpdateWarning(resp.Header)
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

func (c *client) resolve(p string) *url.URL {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + p
	return &u
}

func logRequest(ctx context.Context, req *http.Request, body []byte) {
	e := zerolog.Ctx(ctx).Debug().
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Str("accept", req.Header.Get("Accept")).
		Str("content_type", req.Header.Get("Content-Type")).
		Str("user_agent", req.Header.Get("User-Agent"))
	if token := req.Header.Get("Authorization"); token != "" {
		e = e.Str("authorization", redactSecret(token))
	}
	if token := req.Header.Get("Tollbit-Token"); token != "" {
		e = e.Str("tollbit_token", redactSecret(token))
	}
	if ua := req.Header.Get("Tollbit-User-Agent"); ua != "" {
		e = e.Str("tollbit_user_agent", ua)
	}
	if len(body) > 0 {
		e = e.Str("request_body", strings.TrimSpace(string(body)))
	}
	e.Msg("tollbit request")
}

func logResponse(ctx context.Context, resp *http.Response) {
	zerolog.Ctx(ctx).Debug().
		Int("status_code", resp.StatusCode).
		Str("status", resp.Status).
		Msg("tollbit response")
}

func redactSecret(value string) string {
	if len(value) <= 10 {
		return value
	}
	return value[:6] + "..." + value[len(value)-4:]
}
