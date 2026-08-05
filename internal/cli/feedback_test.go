package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tollbit/cli/internal/client/tollbit"
)

func TestParseMetadataFlags(t *testing.T) {
	meta, err := parseMetadataFlags([]string{"source=cli", " feature = search ", "empty="})
	if err != nil {
		t.Fatal(err)
	}
	if meta["source"] != "cli" || meta["feature"] != "search" || meta["empty"] != "" {
		t.Fatalf("unexpected metadata: %#v", meta)
	}

	_, err = parseMetadataFlags([]string{"no-equals"})
	if err == nil || !strings.Contains(err.Error(), "key=value") {
		t.Fatalf("expected key=value usage error, got %v", err)
	}
}

func TestRunFeedbackAccepted(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/agent/v1/tokens/identity" {
			t.Fatalf("unexpected auth request: %s %s", r.Method, r.URL.RequestURI())
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/agents/v1/feedback" {
			t.Fatalf("unexpected gateway request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		var body tollbit.SubmitFeedbackRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Message != "CLI smoke" {
			t.Fatalf("unexpected message: %q", body.Message)
		}
		if body.Rating == nil || *body.Rating != 5 {
			t.Fatalf("unexpected rating: %#v", body.Rating)
		}
		if body.Category != "deploy" {
			t.Fatalf("unexpected category: %q", body.Category)
		}
		if body.Metadata["source"] != "cli-test" {
			t.Fatalf("unexpected metadata: %#v", body.Metadata)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(tollbit.SubmitFeedbackResponse{Accepted: true})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{
		"feedback", "CLI smoke",
		"--rating", "5",
		"--category", "deploy",
		"--metadata", "source=cli-test",
	}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if want := "Feedback accepted."; !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected stdout to contain %q, got %q", want, stdout.String())
	}
}

func TestRunFeedbackJSON(t *testing.T) {
	token := testAgentJWT(t)
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	}))
	defer authSrv.Close()

	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(tollbit.SubmitFeedbackResponse{Accepted: true})
	}))
	defer gatewaySrv.Close()

	t.Setenv(testAuthBaseURLEnvVar, authSrv.URL)
	t.Setenv(testGatewayBaseURLEnvVar, gatewaySrv.URL)
	t.Setenv(testCredentialsStorageDirEnvVar, t.TempDir())
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"feedback", "json please", "--json"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	var resp tollbit.SubmitFeedbackResponse
	if err := json.NewDecoder(&stdout).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Accepted {
		t.Fatal("expected accepted true")
	}
}

func TestRunFeedbackUsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := executeTestCommand([]string{"feedback"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected usage exit 2, got %d stderr=%q", code, stderr.String())
	}

	code = executeTestCommand([]string{"feedback", "hi", "--rating", "9"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected usage exit 2 for bad rating, got %d stderr=%q", code, stderr.String())
	}
}
