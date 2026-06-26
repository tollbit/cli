package errorsx

import (
	"fmt"
	"net/http"
	"strings"
)

// GenericHTTPError represents an HTTP error response that could not be parsed
// as structured ProblemJSON. Body contains the trimmed raw response body.
type GenericHTTPError struct {
	Status     string
	StatusCode int
	RequestID  string
	Body       string
}

// ParseGenericHTTPError builds a GenericHTTPError from a non-ProblemJSON HTTP
// error response. It does not attempt to parse the body as ProblemJSON.
func ParseGenericHTTPError(status string, statusCode int, headers http.Header, body []byte) GenericHTTPError {
	return GenericHTTPError{
		Status:     status,
		StatusCode: statusCode,
		RequestID:  requestIDFromHeaders(headers),
		Body:       strings.TrimSpace(string(body)),
	}
}

func (e GenericHTTPError) Error() string {
	message := "request failed: " + e.statusText()
	if e.Body != "" {
		message += ": " + e.Body
	}
	if e.RequestID != "" {
		message += " (requestId: " + e.RequestID + ")"
	}
	return message
}

func (e GenericHTTPError) statusText() string {
	if e.Status != "" {
		return e.Status
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("%d", e.StatusCode)
	}
	return "unknown status"
}

// requestIDFromHeaders extracts the best-effort request identifier returned by
// upstream services.
//
// TODO: Consolidate request ID response header names in go-service-commons and use those shared constants here.
func requestIDFromHeaders(headers http.Header) string {
	keys := []string{
		"X-Request-ID",
		"X-Request-Id",
		"Request-ID",
		"Request-Id",
		"X-Correlation-ID",
		"X-Correlation-Id",
	}
	for _, key := range keys {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			return value
		}
	}
	for headerKey, values := range headers {
		for _, key := range keys {
			if !strings.EqualFold(headerKey, key) {
				continue
			}
			for _, value := range values {
				if value = strings.TrimSpace(value); value != "" {
					return value
				}
			}
		}
	}
	return ""
}
