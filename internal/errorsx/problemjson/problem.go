package problemjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type ErrorCode string

const (
	// ErrorCodeOboRequired means the caller needs an on-behalf-of association
	// before the operation can proceed.
	ErrorCodeOboRequired ErrorCode = "obo_required"
	// ErrorCodeUserAgentNotRegistered means the user agent is not registered for
	// content access token creation.
	ErrorCodeUserAgentNotRegistered ErrorCode = "user_agent_not_registered"
	// ErrorCodeCLIUpdateRequired means the backend rejected this CLI version as
	// below the minimum supported version; the user must update to continue.
	ErrorCodeCLIUpdateRequired ErrorCode = "cli_update_required"
)

// Problem is the CLI-local representation of the ProblemJSON shape returned by
// TollBit services. It intentionally mirrors the wire format without importing
// private service packages.
type Problem struct {
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Status    int        `json:"status"`
	Detail    *string    `json:"detail,omitempty"`
	Instance  *string    `json:"instance,omitempty"`
	RequestID *string    `json:"requestId,omitempty"`
	Code      *ErrorCode `json:"code,omitempty"`

	AdditionalProperties map[string]json.RawMessage `json:"-"`
}

type OBORequirement struct {
	Org  bool
	User bool
}

// Parse decodes a response body as ProblemJSON. Unknown top-level fields are
// preserved in AdditionalProperties. Unknown error code values are preserved as
// ErrorCode values for forward compatibility.
func Parse(body []byte) (Problem, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return Problem{}, errors.New("problem json body is empty")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Problem{}, err
	}

	var problem Problem
	if err := json.Unmarshal(body, &problem); err != nil {
		return Problem{}, err
	}
	if problem.Status == 0 && problem.Title == "" && problem.Detail == nil {
		return Problem{}, errors.New("not problem json")
	}

	for _, key := range []string{"type", "title", "status", "detail", "instance", "requestId", "code"} {
		delete(raw, key)
	}
	problem.AdditionalProperties = raw

	return problem, nil
}

// Error returns a concise user-facing error string including requestId when one
// is available.
func (p Problem) Error() string {
	status := p.statusText()
	message := p.DetailOrTitle()
	if message == "" {
		message = "request failed"
	}
	if p.RequestID != nil && *p.RequestID != "" {
		return fmt.Sprintf("%s: %s (requestId: %s)", status, message, *p.RequestID)
	}
	return fmt.Sprintf("%s: %s", status, message)
}

// DetailOrTitle returns the specific problem detail when present, otherwise the
// problem title.
func (p Problem) DetailOrTitle() string {
	if p.Detail != nil && *p.Detail != "" {
		return *p.Detail
	}
	return p.Title
}

// IsOBORequired reports whether the problem has the obo_required error code.
func (p Problem) IsOBORequired() bool {
	return p.Code != nil && *p.Code == ErrorCodeOboRequired
}

// IsUserAgentNotRegistered reports whether the problem has the
// user_agent_not_registered error code.
func (p Problem) IsUserAgentNotRegistered() bool {
	return p.Code != nil && *p.Code == ErrorCodeUserAgentNotRegistered
}

// IsCLIUpdateRequired reports whether the problem has the cli_update_required
// error code.
func (p Problem) IsCLIUpdateRequired() bool {
	return p.Code != nil && *p.Code == ErrorCodeCLIUpdateRequired
}

// StringProperty returns a top-level additional property as a string.
func (p Problem) StringProperty(key string) (string, bool) {
	raw, ok := p.AdditionalProperties[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// RequiredOBO parses the required.obo.org/user extension returned with
// obo_required problems.
func (p Problem) RequiredOBO() (OBORequirement, bool) {
	raw, ok := p.AdditionalProperties["required"]
	if !ok {
		return OBORequirement{}, false
	}

	var value struct {
		OBO OBORequirement `json:"obo"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return OBORequirement{}, false
	}
	return value.OBO, true
}

func (p Problem) statusText() string {
	if p.Status != 0 && p.Title != "" {
		return fmt.Sprintf("%d %s", p.Status, p.Title)
	}
	if p.Title != "" {
		return p.Title
	}
	if p.Status != 0 {
		return fmt.Sprintf("%d", p.Status)
	}
	return "request failed"
}
