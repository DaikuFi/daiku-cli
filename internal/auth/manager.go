package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/securefile"
)

var ErrRefreshInProgress = errors.New("another daiku process is refreshing this profile")

type Manager struct {
	Store              credentials.Store
	OAuth              *Client
	LockDir            string
	DisableProcessLock bool
	mu                 sync.Mutex
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
	release, err := m.acquire(profile)
	if err != nil {
		return "", err
	}
	defer release()
	token, err = m.Store.Get(profile)
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

func (m *Manager) acquire(profile string) (func(), error) {
	if m.DisableProcessLock {
		return func() {}, nil
	}
	dir := m.LockDir
	if dir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("refresh lock unavailable: %w", err)
		}
		dir = filepath.Join(cache, "daiku", "locks")
	}
	if err := securefile.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("refresh lock unavailable: %w", err)
	}
	profileHash := sha256.Sum256([]byte(profile))
	path := filepath.Join(dir, fmt.Sprintf("%x.lock", profileHash))
	file, err := securefile.OpenLock(path)
	if err != nil {
		return nil, fmt.Errorf("refresh lock unavailable: %w", err)
	}
	if err = tryAdvisoryLock(file); err != nil {
		file.Close()
		return nil, err
	}
	return func() { _ = unlockAdvisory(file); _ = file.Close() }, nil
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
