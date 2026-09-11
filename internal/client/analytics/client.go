package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/errorsx"
	"github.com/tollbit/cli/internal/tokens/agent"
)

const (
	queryPath  = "/analytics/agent/v1/query"
	schemaPath = queryPath + "/schema"
)

type (
	Config struct {
		BaseURL string
	}

	Client interface {
		Query(context.Context, QueryRequest, agent.Token) (QueryResponse, error)
		Schema(context.Context, agent.Token) ([]QueryTable, error)
	}

	client struct {
		baseURL *url.URL
		http    *http.Client
	}

	QueryRequest struct {
		SQL string `json:"sql"`
	}

	QueryColumn struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	QueryResponse struct {
		Columns []QueryColumn `json:"columns"`
		Rows    [][]any       `json:"rows"`
	}

	QueryTable struct {
		Name    string        `json:"name"`
		Columns []QueryColumn `json:"columns"`
	}
)

var _ Client = (*client)(nil)

func NewClient(cfg Config) (Client, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("analytics base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &client{
		baseURL: parsed,
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *client) Query(ctx context.Context, request QueryRequest, token agent.Token) (QueryResponse, error) {
	if strings.TrimSpace(token.RawToken) == "" {
		return QueryResponse{}, errors.New("agent token is required")
	}
	if err := token.Validate(); err != nil {
		return QueryResponse{}, err
	}

	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(request); err != nil {
		return QueryResponse{}, err
	}
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + queryPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return QueryResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.RawToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return QueryResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return QueryResponse{}, errorsx.ParseResponseError(ctx, resp.Status, resp.StatusCode, resp.Header, body)
	}

	var result QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return QueryResponse{}, err
	}
	return result, nil
}

func (c *client) Schema(ctx context.Context, token agent.Token) ([]QueryTable, error) {
	if strings.TrimSpace(token.RawToken) == "" {
		return nil, errors.New("agent token is required")
	}
	if err := token.Validate(); err != nil {
		return nil, err
	}

	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + schemaPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.RawToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errorsx.ParseResponseError(ctx, resp.Status, resp.StatusCode, resp.Header, body)
	}

	var result []QueryTable
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
