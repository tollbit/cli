package agenttoken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tollbit/cli/internal/tokens/agent"
)

const refreshTokenExpirySkew = time.Minute

type credentials struct {
	AgentToken            agent.Token
	RefreshToken          string
	RefreshTokenExpiresAt string
}

type refreshCredential struct {
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresAt string `json:"refresh_token_expires_at,omitempty"`
}

func (cred refreshCredential) expired() (bool, error) {
	expiresAt := strings.TrimSpace(cred.RefreshTokenExpiresAt)
	if expiresAt == "" {
		return false, nil
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true, fmt.Errorf("parse refresh token expires at: %w", err)
	}
	return !parsed.After(time.Now().Add(refreshTokenExpirySkew)), nil
}

func (m *CredentialManager) saveCredentials(ctx context.Context, creds credentials) error {
	if err := m.write(ctx, creds.AgentToken); err != nil {
		return err
	}

	refreshToken := strings.TrimSpace(creds.RefreshToken)
	if refreshToken == "" {
		return m.clearRefreshCredential(ctx)
	}

	return m.writeJSON(ctx, m.refreshPath, refreshCredential{
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: strings.TrimSpace(creds.RefreshTokenExpiresAt),
	})
}

func (m *CredentialManager) cachedAgentToken(ctx context.Context) (agent.Token, bool, error) {
	if err := ctx.Err(); err != nil {
		return agent.Token{}, false, err
	}
	raw, err := os.ReadFile(m.tokenPath)
	if os.IsNotExist(err) {
		return agent.Token{}, false, nil
	}
	if err != nil {
		return agent.Token{}, false, fmt.Errorf("read agent token credential: %w", err)
	}
	token := agent.Token{RawToken: strings.TrimSpace(string(raw))}
	err = token.Validate()
	return token, true, err
}

func (m *CredentialManager) write(ctx context.Context, token agent.Token) error {
	return m.writeRaw(ctx, m.tokenPath, []byte(token.RawToken))
}

func (m *CredentialManager) clearAgentToken(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(m.tokenPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear agent token credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) clearRefreshCredential(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(m.refreshPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("clear refresh token credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) writeJSON(ctx context.Context, path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return m.writeRaw(ctx, path, b)
}

func (m *CredentialManager) writeRaw(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create agent credential directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(path), ".agent-credential-*.tmp")
	if err != nil {
		return fmt.Errorf("create agent credential temp file: %w", err)
	}
	tmp := f.Name()
	defer func() {
		_ = os.Remove(tmp)
	}()

	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("set agent credential temp file permissions: %w", err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("write agent credential: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close agent credential temp file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save agent credential: %w", err)
	}
	return nil
}

func (m *CredentialManager) readRefreshCredential(ctx context.Context) (refreshCredential, bool, error) {
	if err := ctx.Err(); err != nil {
		return refreshCredential{}, false, err
	}
	raw, err := os.ReadFile(m.refreshPath)
	if os.IsNotExist(err) {
		return refreshCredential{}, false, nil
	}
	if err != nil {
		return refreshCredential{}, false, fmt.Errorf("read refresh token credential: %w", err)
	}
	var cred refreshCredential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return refreshCredential{}, false, fmt.Errorf("parse refresh token credential: %w", err)
	}
	cred.RefreshToken = strings.TrimSpace(cred.RefreshToken)
	if cred.RefreshToken == "" {
		return refreshCredential{}, false, errors.New("refresh token credential missing token")
	}
	return cred, true, nil
}
