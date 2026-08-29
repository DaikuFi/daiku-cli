package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/DaikuFi/daiku-cli/internal/securefile"
	"github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("credentials not found")
var validProfile = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func safeProfile(profile string) error {
	if !validProfile.MatchString(profile) || profile == "." || profile == ".." {
		return errors.New("invalid credential profile")
	}
	return nil
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	Scope        string `json:"scope,omitempty"`
}

type Store interface {
	Get(string) (Token, error)
	Put(string, Token) error
	Delete(string) error
}

type Keyring struct{ Service string }

func (k Keyring) service() string {
	if k.Service != "" {
		return k.Service
	}
	return "daiku-cli"
}
func (k Keyring) Get(profile string) (Token, error) {
	if err := safeProfile(profile); err != nil {
		return Token{}, err
	}
	raw, err := keyring.Get(k.service(), profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return Token{}, ErrNotFound
	}
	if err != nil {
		return Token{}, fmt.Errorf("keychain unavailable: %w", err)
	}
	var token Token
	if json.Unmarshal([]byte(raw), &token) != nil {
		return Token{}, errors.New("stored credentials are invalid")
	}
	return token, nil
}
func (k Keyring) Put(profile string, token Token) error {
	if err := safeProfile(profile); err != nil {
		return err
	}
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	if err = keyring.Set(k.service(), profile, string(raw)); err != nil {
		return fmt.Errorf("keychain unavailable: %w", err)
	}
	return nil
}
func (k Keyring) Delete(profile string) error {
	if err := safeProfile(profile); err != nil {
		return err
	}
	err := keyring.Delete(k.service(), profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keychain unavailable: %w", err)
	}
	return nil
}

// FileStore is an explicit opt-in fallback. It never activates automatically.
type FileStore struct{ Dir string }

func (f FileStore) path(profile string) string { return filepath.Join(f.Dir, profile+".json") }
func (f FileStore) Get(profile string) (Token, error) {
	if err := safeProfile(profile); err != nil {
		return Token{}, err
	}
	var t Token
	p := f.path(profile)
	data, err := securefile.Read(p)
	if errors.Is(err, os.ErrNotExist) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if json.Unmarshal(data, &t) != nil {
		return t, errors.New("stored credentials are invalid")
	}
	return t, nil
}
func (f FileStore) Put(profile string, t Token) error {
	if err := safeProfile(profile); err != nil {
		return err
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return securefile.Write(f.path(profile), data)
}
func (f FileStore) Delete(profile string) error {
	if err := safeProfile(profile); err != nil {
		return err
	}
	return securefile.Remove(f.path(profile))
}
