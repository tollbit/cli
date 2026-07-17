package errorsx

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/tollbit/cli/internal/errorsx/problemjson"
)

func TestParseResponseErrorReturnsProblemJSON(t *testing.T) {
	body := []byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"detail": "on-behalf-of org claim is required",
		"requestId": "body-req-id",
		"code": "obo_required"
	}`)

	err := ParseResponseError(context.Background(), "403 Forbidden", http.StatusForbidden, http.Header{"X-Request-Id": {"header-req-id"}}, body)

	var problem problemjson.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("expected Problem, got %T", err)
	}
	if !problem.IsOBORequired() {
		t.Fatal("expected OBO required")
	}
	if problem.RequestID == nil || *problem.RequestID != "body-req-id" {
		t.Fatalf("requestId = %#v", problem.RequestID)
	}
	if got := err.Error(); got != "403 Forbidden: on-behalf-of org claim is required (requestId: body-req-id)" {
		t.Fatalf("error = %q", got)
	}
}

func TestParseResponseErrorAddsHeaderRequestIDToProblemJSON(t *testing.T) {
	body := []byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"detail": "access denied"
	}`)

	err := ParseResponseError(context.Background(), "403 Forbidden", http.StatusForbidden, http.Header{"X-Request-Id": {"header-req-id"}}, body)

	var problem problemjson.Problem
	if !errors.As(err, &problem) {
		t.Fatalf("expected Problem, got %T", err)
	}
	if problem.RequestID == nil || *problem.RequestID != "header-req-id" {
		t.Fatalf("requestId = %#v", problem.RequestID)
	}
}

func TestParseResponseErrorReturnsGenericForTextBody(t *testing.T) {
	err := ParseResponseError(context.Background(), "502 Bad Gateway", http.StatusBadGateway, http.Header{"X-Request-ID": {"req-123"}}, []byte("upstream failed"))

	var httpErr GenericHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected GenericHTTPError, got %T", err)
	}
	if httpErr.RequestID != "req-123" {
		t.Fatalf("requestId = %q", httpErr.RequestID)
	}
	if got := err.Error(); got != "request failed: 502 Bad Gateway: upstream failed (requestId: req-123)" {
		t.Fatalf("error = %q", got)
	}
}

func TestParseResponseErrorReturnsGenericForNonProblemJSON(t *testing.T) {
	err := ParseResponseError(context.Background(), "400 Bad Request", http.StatusBadRequest, nil, []byte(`{"foo":"bar"}`))

	var httpErr GenericHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected GenericHTTPError, got %T", err)
	}
	if got := err.Error(); got != `request failed: 400 Bad Request: {"foo":"bar"}` {
		t.Fatalf("error = %q", got)
	}
}

func TestParseResponseErrorReturnsGenericForEmptyBody(t *testing.T) {
	err := ParseResponseError(context.Background(), "404 Not Found", http.StatusNotFound, http.Header{"Request-Id": {"req-123"}}, nil)

	var httpErr GenericHTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected GenericHTTPError, got %T", err)
	}
	if got := err.Error(); got != "request failed: 404 Not Found (requestId: req-123)" {
		t.Fatalf("error = %q", got)
	}
}

func TestParseGenericHTTPErrorDoesNotParseProblemJSON(t *testing.T) {
	body := []byte(`{
		"type": "about:blank",
		"title": "Forbidden",
		"status": 403,
		"detail": "access denied"
	}`)

	err := ParseGenericHTTPError("403 Forbidden", http.StatusForbidden, http.Header{"X-Request-Id": {"req-123"}}, body)

	if err.RequestID != "req-123" {
		t.Fatalf("requestId = %q", err.RequestID)
	}
	if err.Body != string(body) {
		t.Fatalf("body = %q", err.Body)
	}
	if got := err.Error(); got != "request failed: 403 Forbidden: "+string(body)+" (requestId: req-123)" {
		t.Fatalf("error = %q", got)
	}
}
