package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/credentials"
)

type Manager struct {
	Store credentials.Store
	OAuth *Client
	mu    sync.Mutex
}

func (m *Manager) AccessToken(ctx context.Context, profile string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	token, err := m.Store.Get(profile)
	if err != nil {
		return "", err
	}
	if token.ExpiresAt == 0 || time.Now().Unix() < token.ExpiresAt-30 {
		return token.AccessToken, nil
	}
	fresh, err := m.OAuth.Refresh(ctx, token.RefreshToken)
	if err != nil {
		return "", err
	}
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = token.RefreshToken
	}
	if err = m.Store.Put(profile, fresh); err != nil {
		return "", err
	}
	return fresh.AccessToken, nil
}
func (m *Manager) Logout(ctx context.Context, profile string) error {
	token, err := m.Store.Get(profile)
	if err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return err
	}
	if err == nil {
		if revokeErr := m.OAuth.Revoke(ctx, token.RefreshToken); revokeErr != nil {
			return revokeErr
		}
	}
	return m.Store.Delete(profile)
}
