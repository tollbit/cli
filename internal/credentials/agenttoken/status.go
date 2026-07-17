package agenttoken

import (
	"time"

	"github.com/tollbit/cli/internal/tokens/agent"
)

type OBOStatus struct {
	Source string `json:"source"`
	User   string `json:"user"`
	Org    string `json:"org"`
}

type TokenStatus struct {
	Configured bool       `json:"configured"`
	Valid      *bool      `json:"valid,omitempty"`
	Error      string     `json:"error,omitempty"`
	Subject    string     `json:"subject,omitempty"`
	ExpiresAt  string     `json:"expires_at,omitempty"`
	OBO        *OBOStatus `json:"obo,omitempty"`
}

func Status(token agent.Token, exists bool, validationErr error) TokenStatus {
	status := TokenStatus{Configured: exists}
	if !exists {
		return status
	}
	valid := validationErr == nil
	status.Valid = &valid
	if validationErr != nil {
		status.Error = validationErr.Error()
	}
	if claims, err := token.Claims(); err == nil {
		status.Subject = claims.Subject
		if claims.ExpiresAt != nil {
			status.ExpiresAt = claims.ExpiresAt.Time.UTC().Format(time.RFC3339)
		}
		if claims.OBO != nil {
			status.OBO = &OBOStatus{
				Source: claims.OBO.Source,
				User:   claims.OBO.User,
				Org:    claims.OBO.Org,
			}
		}
	}
	return status
}
