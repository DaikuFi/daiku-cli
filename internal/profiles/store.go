package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DaikuFi/daiku-cli/internal/securefile"
)

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var validHousehold = regexp.MustCompile(`^hsh_[0-9a-f]{32}$`)

type Profile struct {
	APIURL    string `json:"api_url"`
	Household string `json:"household,omitempty"`
}

type Config struct {
	Current  string             `json:"current,omitempty"`
	Profiles map[string]Profile `json:"profiles"`
}

type Store struct{ Path string }

func ValidateName(name string) error {
	if !validName.MatchString(name) || name == "." || name == ".." {
		return errors.New("profile name must contain only letters, numbers, '.', '_' or '-'")
	}
	return nil
}

func (s Store) Load() (Config, error) {
	cfg := Config{Profiles: map[string]Profile{}}
	data, err := securefile.Read(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read profile configuration: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, errors.New("profile configuration is invalid")
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	for name, profile := range cfg.Profiles {
		if ValidateName(name) != nil {
			return Config{}, errors.New("profile configuration contains an invalid name")
		}
		if _, err := NormalizeAPIURL(profile.APIURL); err != nil {
			return Config{}, errors.New("profile configuration contains an invalid API URL")
		}
		if profile.Household != "" && !validHousehold.MatchString(profile.Household) {
			return Config{}, errors.New("profile configuration contains an invalid household ID")
		}
	}
	if cfg.Current != "" {
		if ValidateName(cfg.Current) != nil {
			return Config{}, errors.New("profile configuration contains an invalid current profile")
		}
		if _, ok := cfg.Profiles[cfg.Current]; !ok {
			return Config{}, errors.New("current profile does not exist")
		}
	}
	return cfg, nil
}

func (s Store) Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return securefile.Write(s.Path, data)
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daiku", "config.json"), nil
}

func NormalizeAPIURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "https://api.daiku.app/api/v1/", nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("invalid API URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(parsed.Path, "/api/v1/") {
		return "", errors.New("API URL must be an absolute /api/v1/ URL")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return "", errors.New("API URL must use HTTPS")
	}
	return parsed.String(), nil
}
