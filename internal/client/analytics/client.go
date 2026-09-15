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
		Schema(context.Context, agent.Token) (SchemaResponse, error)
	}

	client struct {
		baseURL *url.URL
		http    *http.Client
	}

	QueryRequest struct {
		SQL string `json:"sql"`
	}

	QueryColumn struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		Description string   `json:"description,omitempty"`
		Values      []string `json:"values,omitempty"`
	}

	// QueryMeta is sent by servers that report result metadata. Absent on
	// older servers, so it is a pointer and omitted when nil.
	QueryMeta struct {
		RowCount     int   `json:"row_count"`
		Truncated    bool  `json:"truncated"`
		BytesScanned int64 `json:"bytes_scanned"`
		DurationMs   int64 `json:"duration_ms"`
	}

	QueryResponse struct {
		Columns []QueryColumn `json:"columns"`
		Rows    [][]any       `json:"rows"`
		Meta    *QueryMeta    `json:"meta,omitempty"`
	}

	QueryTable struct {
		Name        string        `json:"name"`
		Description string        `json:"description,omitempty"`
		Columns     []QueryColumn `json:"columns"`
		Clustering  []string      `json:"clustering,omitempty"`
	}

	Limit struct {
		Value       json.Number `json:"value"`
		Unit        string      `json:"unit"`
		Description string      `json:"description,omitempty"`
	}

	// SchemaResponse is the schema object. Older servers return a bare array
	// of tables; Schema accepts both and always returns this shape.
	SchemaResponse struct {
		Dialect string           `json:"dialect,omitempty"`
		Tables  []QueryTable     `json:"tables"`
		Limits  map[string]Limit `json:"limits,omitempty"`
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

func (c *client) Schema(ctx context.Context, token agent.Token) (SchemaResponse, error) {
	if strings.TrimSpace(token.RawToken) == "" {
		return SchemaResponse{}, errors.New("agent token is required")
	}
	if err := token.Validate(); err != nil {
		return SchemaResponse{}, err
	}

	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + schemaPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return SchemaResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.RawToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return SchemaResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return SchemaResponse{}, errorsx.ParseResponseError(ctx, resp.Status, resp.StatusCode, resp.Header, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SchemaResponse{}, err
	}
	return decodeSchema(body)
}

// decodeSchema accepts the schema object or the older bare array of tables.
func decodeSchema(body []byte) (SchemaResponse, error) {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var tables []QueryTable
		if err := json.Unmarshal(trimmed, &tables); err != nil {
			return SchemaResponse{}, err
		}
		return SchemaResponse{Tables: tables}, nil
	}
	var result SchemaResponse
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return SchemaResponse{}, err
	}
	return result, nil
}
