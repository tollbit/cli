package agenttoken

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tollbit/cli/internal/agentauth"
	"github.com/tollbit/cli/internal/client/auth"
	"github.com/tollbit/cli/internal/tokens/agent"
)

func (m *CredentialManager) CompletePendingConsent(ctx context.Context, pending agentauth.PendingConsent, resp auth.AgentTokenResponse) (agent.Token, error) {
	token, err := m.saveAgentTokenResponse(ctx, resp, pending.UseRefreshTokens)
	if err != nil {
		return agent.Token{}, err
	}
	if err := m.WriteIdentity(ctx, pending.AgentIdentity); err != nil {
		return agent.Token{}, err
	}
	if err := m.ClearPendingConsent(ctx); err != nil {
		return agent.Token{}, err
	}
	return token, nil
}

func (m *CredentialManager) savePendingConsent(ctx context.Context, pending agentauth.PendingConsent) error {
	if strings.TrimSpace(pending.ChallengeID) == "" {
		return fmt.Errorf("pending consent challenge id is required")
	}
	if strings.TrimSpace(pending.AgentIdentity.Name) == "" {
		return fmt.Errorf("pending consent agent identity is required")
	}
	if strings.TrimSpace(pending.BaseToken) == "" {
		return fmt.Errorf("pending consent base token is required")
	}
	if strings.TrimSpace(pending.CodeVerifier) == "" {
		return fmt.Errorf("pending consent code verifier is required")
	}
	if pending.Method == agentauth.ConsentMethodAgentConfirmsIcons && len(pending.IconNames) == 0 {
		return fmt.Errorf("pending consent icon names are required")
	}
	return m.writeJSON(ctx, m.pendingPath, pending)
}

func (m *CredentialManager) GetPendingConsent(ctx context.Context) (agentauth.PendingConsent, bool, error) {
	if err := ctx.Err(); err != nil {
		return agentauth.PendingConsent{}, false, err
	}
	raw, err := os.ReadFile(m.pendingPath)
	if os.IsNotExist(err) {
		return agentauth.PendingConsent{}, false, nil
	}
	if err != nil {
		return agentauth.PendingConsent{}, false, fmt.Errorf("read pending auth credential: %w", err)
	}
	var pending agentauth.PendingConsent
	if err := json.Unmarshal(raw, &pending); err != nil {
		return agentauth.PendingConsent{}, false, fmt.Errorf("parse pending auth credential: %w", err)
	}
	if strings.TrimSpace(pending.ChallengeID) == "" {
		return agentauth.PendingConsent{}, false, fmt.Errorf("pending auth credential missing challenge id")
	}
	return pending, true, nil
}

func (m *CredentialManager) ClearPendingConsent(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(m.pendingPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear pending auth credential: %w", err)
	}
	return nil
}
