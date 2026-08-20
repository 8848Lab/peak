package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows os.UserHomeDir() fallback

	if err := SaveToken("pk_live_abc123"); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "pk_live_abc123" {
		t.Errorf("got token %q, want pk_live_abc123", cfg.Token)
	}
}

func TestLoadWithNoTokenReturnsEmptyToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("got token %q, want empty", cfg.Token)
	}
	if cfg.APIBaseURL != DefaultAPIURL {
		t.Errorf("got APIBaseURL %q, want %q", cfg.APIBaseURL, DefaultAPIURL)
	}
}

func TestSaveAndLoadProjectLink(t *testing.T) {
	dir := t.TempDir()
	link := &ProjectLink{ProjectID: "proj-1", OrganizationID: "org-1", Name: "my-app"}

	if err := SaveProjectLink(dir, link); err != nil {
		t.Fatalf("SaveProjectLink: %v", err)
	}

	loaded, err := LoadProjectLink(dir)
	if err != nil {
		t.Fatalf("LoadProjectLink: %v", err)
	}
	if *loaded != *link {
		t.Errorf("got %+v, want %+v", *loaded, *link)
	}
}

func TestLoadProjectLinkMissingReturnsError(t *testing.T) {
	dir := t.TempDir()

	if _, err := LoadProjectLink(dir); err == nil {
		t.Error("expected an error for a directory with no .peak/project.json")
	}
}

func TestEnsureGitignoredAddsEntryWhenGitignoreExists(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("EnsureGitignored: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, ".peak/") {
		t.Errorf(".gitignore content %q does not contain .peak/", content)
	}
	if !strings.Contains(content, "node_modules/") {
		t.Errorf(".gitignore content %q lost its existing entry", content)
	}
}

func TestEnsureGitignoredNoopsWhenAlreadyIgnored(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	original := "node_modules/\n.peak/\n"
	if err := os.WriteFile(gitignorePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("EnsureGitignored: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("got %q, want unchanged %q", string(data), original)
	}
}

func TestEnsureGitignoredNoopsWhenNoGitignoreExists(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("EnsureGitignored: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error("expected EnsureGitignored not to create a .gitignore when none existed")
	}
}
