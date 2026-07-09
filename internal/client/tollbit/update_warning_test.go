package tollbit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tollbit/tollbit-cli/internal/errorsx/problemjson"
	"github.com/tollbit/tollbit-cli/internal/version"
)

func captureUpdateWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	prevWriter := updateWarnWriter
	updateWarnWriter = buf
	updateWarnOnce = sync.Once{}
	t.Cleanup(func() {
		updateWarnWriter = prevWriter
		updateWarnOnce = sync.Once{}
	})
	return buf
}

func TestClientVersionHeaderSent(t *testing.T) {
	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Tollbit-Client")
		if got != version.ClientHeader() {
			t.Errorf("X-Tollbit-Client = %q, want %q", got, version.ClientHeader())
		}
		if !strings.HasPrefix(got, "search-cli/") {
			t.Errorf("X-Tollbit-Client %q missing search-cli/ product prefix", got)
		}
		_ = json.NewEncoder(w).Encode(PagedSearchResultResponse{})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(context.Background(), SearchParams{Query: "q"}, token); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWarningPrintedOncePerProcess(t *testing.T) {
	buf := captureUpdateWarnings(t)
	const warning = "A new version of the TollBit CLI is available. Run: npm update -g @tollbit/cli"

	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Tollbit-Cli-Warning", warning)
		_ = json.NewEncoder(w).Encode(PagedSearchResultResponse{})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := c.Search(context.Background(), SearchParams{Query: "q"}, token); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := buf.String(), warning+"\n"; got != want {
		t.Fatalf("warning output = %q, want it printed exactly once as %q", got, want)
	}
}

func TestNoUpdateWarningHeaderPrintsNothing(t *testing.T) {
	buf := captureUpdateWarnings(t)

	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(PagedSearchResultResponse{})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Search(context.Background(), SearchParams{Query: "q"}, token); err != nil {
		t.Fatal(err)
	}

	if buf.Len() != 0 {
		t.Fatalf("expected no warning output, got %q", buf.String())
	}
}

func TestUpdateRequiredBlocksWithProblemJSON(t *testing.T) {
	const detail = "This version of the TollBit CLI is no longer supported. Run: npm update -g @tollbit/cli"

	token := validAgentToken(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUpgradeRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "about:blank",
			"title":  "Upgrade Required",
			"status": http.StatusUpgradeRequired,
			"detail": detail,
			"code":   "cli_update_required",
		})
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Search(context.Background(), SearchParams{Query: "q"}, token)
	if err == nil {
		t.Fatal("expected error")
	}

	var problem problemjson.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("expected problemjson.Problem, got %T: %v", err, err)
	}
	if problem.Status != http.StatusUpgradeRequired {
		t.Fatalf("problem status = %d, want %d", problem.Status, http.StatusUpgradeRequired)
	}
	if problem.Code == nil || *problem.Code != problemjson.ErrorCode("cli_update_required") {
		t.Fatalf("problem code = %v, want cli_update_required", problem.Code)
	}
	if problem.IsOBORequired() {
		t.Fatal("426 must not be classified as obo_required (would trigger token retry)")
	}
	if !strings.Contains(err.Error(), detail) {
		t.Fatalf("error %q should contain server-provided detail", err.Error())
	}
}
