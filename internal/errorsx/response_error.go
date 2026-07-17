package errorsx

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/tollbit/cli/internal/errorsx/problemjson"
)

// ParseResponseError parses an HTTP error response body.
//
// It prefers structured ProblemJSON when the response body matches the
// CLI-supported ProblemJSON shape. If parsing as ProblemJSON fails, it falls
// back to GenericHTTPError using the raw response body.
func ParseResponseError(ctx context.Context, status string, statusCode int, headers http.Header, body []byte) error {
	if problem, err := problemjson.Parse(body); err == nil {
		if problem.RequestID == nil {
			if requestID := requestIDFromHeaders(headers); requestID != "" {
				problem.RequestID = &requestID
			}
		}
		zerolog.Ctx(ctx).Debug().
			Int("status_code", statusCode).
			Str("status", status).
			Str("error_type", "problem_json").
			Str("problem_title", problem.Title).
			Str("problem_code", problemCode(problem.Code)).
			Msg("parsed HTTP error response")
		return problem
	} else {
		zerolog.Ctx(ctx).Debug().
			Int("status_code", statusCode).
			Str("status", status).
			Str("error_type", "generic_http").
			Err(err).
			Msg("parsed HTTP error response")
	}

	return ParseGenericHTTPError(status, statusCode, headers, body)
}

func problemCode(code *problemjson.ErrorCode) string {
	if code == nil {
		return ""
	}
	return string(*code)
}
