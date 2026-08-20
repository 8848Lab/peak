package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the CLI configuration loaded from ~/.peak/token
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

// ProjectLink records which Himalaya project the current directory deploys
// to, so `peak deploy` doesn't have to ask every time.
type ProjectLink struct {
	ProjectID      string `json:"project_id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
}

const projectLinkFile = ".peak/project.json"

// LoadProjectLink reads dir's project link file, if one exists.
func LoadProjectLink(dir string) (*ProjectLink, error) {
	data, err := os.ReadFile(filepath.Join(dir, projectLinkFile))
	if err != nil {
		return nil, err
	}
	var link ProjectLink
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, err
	}
	return &link, nil
}

// SaveProjectLink writes dir's project link file, creating .peak/ if needed.
func SaveProjectLink(dir string, link *ProjectLink) error {
	linkDir := filepath.Join(dir, ".peak")
	if err := os.MkdirAll(linkDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, projectLinkFile), data, 0600)
}

// EnsureGitignored appends ".peak/" to dir's .gitignore if one exists and
// doesn't already ignore it. No-ops (does not create a .gitignore) if dir
// has no .gitignore at all — peak shouldn't opt a non-git directory into
// git conventions unasked.
func EnsureGitignored(dir string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".peak/" || trimmed == ".peak" || trimmed == "/.peak/" {
			return nil
		}
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ".peak/\n"
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}
