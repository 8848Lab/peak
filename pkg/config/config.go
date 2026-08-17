package config

import (
	"os"
	"path/filepath"
)

// Config holds the CLI configuration loaded from ~/.peak/config.toml
type Config struct {
	Token      string
	APIBaseURL string
}

// DefaultAPIURL is the Himalaya API endpoint
const DefaultAPIURL = "https://api.8848lab.org"

// ConfigDir returns the path to the peak config directory
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".peak"), nil
}

// Load reads config from disk. Returns defaults if not found.
func Load() (*Config, error) {
	cfg := &Config{
		APIBaseURL: DefaultAPIURL,
	}

	dir, err := ConfigDir()
	if err != nil {
		return cfg, nil
	}

	tokenPath := filepath.Join(dir, "token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		// Not logged in yet — that's fine
		return cfg, nil
	}

	cfg.Token = string(data)
	return cfg, nil
}

// SaveToken persists the auth token to disk
func SaveToken(token string) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "token"), []byte(token), 0600)
}
