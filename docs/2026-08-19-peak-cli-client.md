# Peak CLI — Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace every mocked `peak` command (`login`, `deploy`, `logs`, `status`) with real behavior against the Himalaya API, and make the project actually build (`cmd/peak/main.go` doesn't exist yet).

**Architecture:** `internal/api/client.go` becomes a real `net/http` client for every endpoint the backend plan adds. `peak login` drives the OAuth-device-code flow (`/auth/device/start` → open browser → poll `/auth/device/poll`) inside the existing Bubbletea step-list UI. `peak deploy` tars the current directory (new `internal/archive` package), uploads it (`POST /projects/{id}/deployments/local`), and polls `GET /deployments/{id}` until it leaves `queued`/`building`. `peak logs`/`peak status` become simple real API calls, sharing one new helper (`resolveDeploymentID`) that resolves "no ID given" to the current directory's linked project's latest deployment.

**Tech Stack:** Go 1.22, Cobra, Bubbletea/Bubbles/Lipgloss (already vendored), `net/http` (stdlib, no new HTTP client library), `github.com/cli/browser` (new, for opening the login URL), `github.com/sabhiram/go-gitignore` (new, for archive exclusion).

**Spec:** `docs/superpowers/specs/2026-08-19-peak-cli-design.md` in the Himalaya repo (`D:/Coding/8848 Lab/Himalaya`) — this plan implements Sub-project 2 in full. It depends on the sibling plan `2026-08-19-peak-cli-backend.md` (same spec, Himalaya repo) for every endpoint it calls; that plan should be merged (or at least runnable locally via `docker compose up -d api worker redis postgres` from the Himalaya repo root) before Task 5 of this plan is exercised end-to-end.

## Global Constraints

- Every wire type in this plan (JSON field names, the device-flow response shape, `DeploymentRead.source`, etc.) matches the backend plan's schemas exactly — see that plan's Task 4 (`DeviceStartResponse`/`DevicePollResponse`), Task 6 (`DeploymentRead`), and Task 5 (`ProjectRead`). The query-string/JSON field is `user_code` everywhere, not `code` (the backend plan's Global Constraints section resolves an inconsistency in the spec's own prose in favor of `user_code`; this plan follows that resolution).
- The device-code poll `interval` and `expires_in` returned by `POST /auth/device/start` are authoritative — the CLI must use the server-supplied values for its poll cadence and its own give-up deadline, not hardcode `3`/`600` a second time (even though those happen to be the current server-side constants).
- No Go tests exist in this repo today. Every new file in this plan gets a `_test.go` using `net/http/httptest.Server` to stand in for Himalaya — never a live network call in a test.
- Bubbletea view/update logic is not meaningfully unit-testable and isn't tested directly in this plan — every real decision (what HTTP call to make, how to parse a response, what state to transition to) lives in plain functions (`internal/api`, `internal/archive`, `pkg/config`, `resolveDeploymentID`, `resolveProjectLink`) that the tests do reach.
- New dependencies are added via `go get <module>@latest` followed by `go mod tidy`, letting Go resolve and pin the real current versions — this plan does not fabricate `go.sum` hashes or version numbers.

---

### Task 1: `cmd/peak/main.go` — fix the build

**Files:**
- Create: `cmd/peak/main.go`

**Interfaces:**
- Consumes: `internal/cli.Execute()` (already exists, `internal/cli/root.go`).
- Produces: a working `go build -o peak ./cmd/peak`.

- [ ] **Step 1: Write the entrypoint**

`cmd/peak/main.go`:
```go
package main

import "github.com/8848lab/peak/internal/cli"

func main() {
	cli.Execute()
}
```

- [ ] **Step 2: Verify the build**

Run: `go build -o peak ./cmd/peak`
Expected: builds successfully (no errors) — this is the "test" for this task, since there's no unit to test yet, just a missing entrypoint. `go vet ./...` should also pass.

- [ ] **Step 3: Commit**

```bash
git add cmd/peak/main.go
git commit -m "feat: add missing cmd/peak/main.go entrypoint"
```

---

### Task 2: `pkg/config` — project link file and `.gitignore` helper

**Files:**
- Modify: `pkg/config/config.go`
- Create: `pkg/config/config_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `ProjectLink{ProjectID, OrganizationID, Name string}`; `LoadProjectLink(dir string) (*ProjectLink, error)`; `SaveProjectLink(dir string, link *ProjectLink) error` (writes `<dir>/.peak/project.json`); `EnsureGitignored(dir string) error` (appends `.peak/` to `<dir>/.gitignore` if one exists and doesn't already ignore it; no-ops if there's no `.gitignore` at all). Task 6 (`peak deploy`) consumes all three.

- [ ] **Step 1: Write the failing tests**

`pkg/config/config_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/config/... -v`
Expected: `TestSaveAndLoadToken` and `TestLoadWithNoTokenReturnsEmptyToken` PASS already (existing functions); every `ProjectLink`/`EnsureGitignored` test FAILs to compile (`undefined: ProjectLink`, `undefined: SaveProjectLink`, etc.).

- [ ] **Step 3: Implement**

`pkg/config/config.go` (full file):
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/config/... -v`
Expected: PASS (all 7 tests).

- [ ] **Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "feat: add project link file and .gitignore helper to pkg/config"
```

---

### Task 3: `internal/api/client.go` — real HTTP client

**Files:**
- Modify: `internal/api/client.go`
- Create: `internal/api/client_test.go`

**Interfaces:**
- Consumes: nothing new (stdlib `net/http`, `encoding/json`, `mime/multipart`).
- Produces: `NewClient(baseURL, token string) *Client` (unchanged signature); `(*Client).StartDeviceAuth() (*DeviceStartResponse, error)`; `(*Client).PollDeviceAuth(deviceCode string) (*DevicePollResponse, error)`; `(*Client).ListOrganizations() ([]Organization, error)`; `(*Client).CreateProject(req CreateProjectRequest) (*Project, error)`; `(*Client).DeployLocal(projectID string, archive io.Reader) (*Deployment, error)`; `(*Client).GetDeployment(id string) (*Deployment, error)`; `(*Client).ListDeployments(projectID string) ([]Deployment, error)`; `(*Client).GetContainerLogs(deploymentID string) (string, error)`; `*APIError` (with `StatusCode int`, `Message string`, `IsAuthError() bool`, `IsNotFound() bool`). Tasks 5, 6, and 7 all consume this client.

The old `DeployRequest`/`DeployResponse`/`Deploy`/`GetStatus` mock methods are removed entirely — they don't match the real upload-based deploy flow this plan implements (see Task 6).

- [ ] **Step 1: Write the failing tests**

`internal/api/client_test.go`:
```go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewClient(server.URL, "pk_live_test-token")
}

func TestStartDeviceAuth(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/device/start" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(DeviceStartResponse{
			DeviceCode:      "dc123",
			UserCode:        "WXYZ-1234",
			VerificationURI: "http://localhost:3000/cli-auth",
			ExpiresIn:       600,
			Interval:        3,
		})
	})

	resp, err := client.StartDeviceAuth()
	if err != nil {
		t.Fatalf("StartDeviceAuth: %v", err)
	}
	if resp.UserCode != "WXYZ-1234" || resp.Interval != 3 {
		t.Errorf("got %+v", resp)
	}
}

func TestPollDeviceAuthSendsDeviceCodeAsQueryParam(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("device_code") != "dc123" {
			t.Errorf("missing/wrong device_code query param: %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(DevicePollResponse{Status: "pending"})
	})

	resp, err := client.PollDeviceAuth("dc123")
	if err != nil {
		t.Fatalf("PollDeviceAuth: %v", err)
	}
	if resp.Status != "pending" {
		t.Errorf("got status %q, want pending", resp.Status)
	}
}

func TestPollDeviceAuthApproved(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DevicePollResponse{
			Status: "approved",
			Token:  "pk_live_realtoken",
			User:   &User{ID: "u1", Email: "dev@example.com"},
		})
	})

	resp, err := client.PollDeviceAuth("dc123")
	if err != nil {
		t.Fatalf("PollDeviceAuth: %v", err)
	}
	if resp.Token != "pk_live_realtoken" || resp.User == nil || resp.User.ID != "u1" {
		t.Errorf("got %+v", resp)
	}
}

func TestListOrganizationsSendsBearerToken(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pk_live_test-token" {
			t.Errorf("got Authorization %q", got)
		}
		json.NewEncoder(w).Encode([]Organization{{ID: "org1", Name: "Acme", Slug: "acme"}})
	})

	orgs, err := client.ListOrganizations()
	if err != nil {
		t.Fatalf("ListOrganizations: %v", err)
	}
	if len(orgs) != 1 || orgs[0].Name != "Acme" {
		t.Errorf("got %+v", orgs)
	}
}

func TestCreateProjectSendsJSONBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req CreateProjectRequest
		json.Unmarshal(body, &req)
		if req.OrganizationID != "org1" || req.Name != "my-app" {
			t.Errorf("got %+v", req)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Project{ID: "proj1", OrganizationID: "org1", Name: "my-app"})
	})

	project, err := client.CreateProject(CreateProjectRequest{OrganizationID: "org1", Name: "my-app"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if project.ID != "proj1" {
		t.Errorf("got %+v", project)
	}
}

func TestDeployLocalUploadsMultipartArchive(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/proj1/deployments/local" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, _, err := r.FormFile("archive")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "fake-tarball-bytes" {
			t.Errorf("got archive content %q", string(data))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Deployment{ID: "dep1", ProjectID: "proj1", Status: "queued", Source: "upload"})
	})

	deployment, err := client.DeployLocal("proj1", strings.NewReader("fake-tarball-bytes"))
	if err != nil {
		t.Fatalf("DeployLocal: %v", err)
	}
	if deployment.ID != "dep1" || deployment.Status != "queued" {
		t.Errorf("got %+v", deployment)
	}
}

func TestGetDeployment(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deployments/dep1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		url := "http://my-app.localhost"
		json.NewEncoder(w).Encode(Deployment{ID: "dep1", Status: "ready", DeploymentURL: &url})
	})

	deployment, err := client.GetDeployment("dep1")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if deployment.Status != "ready" || deployment.DeploymentURL == nil || *deployment.DeploymentURL != "http://my-app.localhost" {
		t.Errorf("got %+v", deployment)
	}
}

func TestListDeployments(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj1/deployments" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]Deployment{{ID: "dep1"}, {ID: "dep2"}})
	})

	deployments, err := client.ListDeployments("proj1")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(deployments) != 2 {
		t.Errorf("got %d deployments", len(deployments))
	}
}

func TestGetContainerLogs(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deployments/dep1/container-logs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"logs": "hello from the container"})
	})

	logs, err := client.GetContainerLogs("dep1")
	if err != nil {
		t.Fatalf("GetContainerLogs: %v", err)
	}
	if logs != "hello from the container" {
		t.Errorf("got %q", logs)
	}
}

func TestErrorResponseParsesDetailMessage(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Deployment not found"})
	})

	_, err := client.GetDeployment("missing")
	if err == nil {
		t.Fatal("expected an error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("got error of type %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 || apiErr.Message != "Deployment not found" {
		t.Errorf("got %+v", apiErr)
	}
	if !apiErr.IsNotFound() {
		t.Error("expected IsNotFound() to be true")
	}
	if apiErr.IsAuthError() {
		t.Error("expected IsAuthError() to be false")
	}
}

func TestErrorResponse401IsAuthError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"detail": "Invalid or expired token"})
	})

	_, err := client.GetDeployment("dep1")
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("got error of type %T, want *APIError", err)
	}
	if !apiErr.IsAuthError() {
		t.Error("expected IsAuthError() to be true")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/... -v`
Expected: FAIL to compile — `DeviceStartResponse`, `DevicePollResponse`, `Organization`, `CreateProjectRequest`, `Project`, `Deployment.Source`/`.DeploymentURL`, `APIError`, and most of the client methods don't exist yet.

- [ ] **Step 3: Implement**

`internal/api/client.go` (full file, replaces the mock version entirely):
```go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Client is the HTTP client for the Himalaya API
type Client struct {
	BaseURL    string
	Token      string
	httpClient *http.Client
}

// NewClient creates an authenticated API client
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// APIError is returned for any non-2xx response. Callers should type-assert
// to it to render auth-expired/not-found/server-error cases differently.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
}

func (e *APIError) IsAuthError() bool { return e.StatusCode == http.StatusUnauthorized }
func (e *APIError) IsNotFound() bool  { return e.StatusCode == http.StatusNotFound }

func apiErrorFromResponse(status int, body []byte) error {
	var parsed struct {
		Detail string `json:"detail"`
	}
	message := "request failed"
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Detail != "" {
		message = parsed.Detail
	}
	return &APIError{StatusCode: status, Message: message}
}

// doJSON sends a JSON request (body may be nil) and decodes a JSON response
// into out (which may be nil for responses with no body, e.g. 204s).
func (c *Client) doJSON(method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return apiErrorFromResponse(resp.StatusCode, respBody)
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

// ── Device auth ──────────────────────────────────────────────────────────

type DeviceStartResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type DevicePollResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
	User   *User  `json:"user"`
}

func (c *Client) StartDeviceAuth() (*DeviceStartResponse, error) {
	var out DeviceStartResponse
	if err := c.doJSON(http.MethodPost, "/auth/device/start", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PollDeviceAuth(deviceCode string) (*DevicePollResponse, error) {
	var out DevicePollResponse
	path := "/auth/device/poll?device_code=" + url.QueryEscape(deviceCode)
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Organizations ────────────────────────────────────────────────────────

type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (c *Client) ListOrganizations() ([]Organization, error) {
	var out []Organization
	if err := c.doJSON(http.MethodGet, "/organizations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── Projects ──────────────────────────────────────────────────────────────

type CreateProjectRequest struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
}

type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
}

func (c *Client) CreateProject(req CreateProjectRequest) (*Project, error) {
	var out Project
	if err := c.doJSON(http.MethodPost, "/projects", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ── Deployments ───────────────────────────────────────────────────────────

type Deployment struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"project_id"`
	Status        string  `json:"status"`
	Source        string  `json:"source"`
	DeploymentURL *string `json:"deployment_url"`
	Logs          *string `json:"logs"`
}

// DeployLocal uploads archive as the deployment source for projectID.
func (c *Client) DeployLocal(projectID string, archive io.Reader) (*Deployment, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("archive", "archive.tar.gz")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, archive); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/projects/"+projectID+"/deployments/local", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, apiErrorFromResponse(resp.StatusCode, respBody)
	}
	var out Deployment
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDeployment(id string) (*Deployment, error) {
	var out Deployment
	if err := c.doJSON(http.MethodGet, "/deployments/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListDeployments(projectID string) ([]Deployment, error) {
	var out []Deployment
	if err := c.doJSON(http.MethodGet, "/projects/"+projectID+"/deployments", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetContainerLogs(deploymentID string) (string, error) {
	var out struct {
		Logs string `json:"logs"`
	}
	if err := c.doJSON(http.MethodGet, "/deployments/"+deploymentID+"/container-logs", nil, &out); err != nil {
		return "", err
	}
	return out.Logs, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/... -v`
Expected: PASS (11 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/api/client.go internal/api/client_test.go
git commit -m "feat: replace mock API client with a real net/http implementation"
```

---

### Task 4: `internal/archive` — tar+gzip with `.gitignore` exclusion

**Files:**
- Create: `internal/archive/archive.go`
- Create: `internal/archive/archive_test.go`
- Modify: `go.mod` (new dependency)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `TarGz(dir string, w io.Writer) error` — writes a gzip-compressed tar of `dir` to `w`, always excluding `.git/`, and excluding whatever `dir`'s own `.gitignore` excludes if one is present. Task 6 (`peak deploy`) consumes this.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/sabhiram/go-gitignore@latest
go mod tidy
```

- [ ] **Step 2: Write the failing test**

`internal/archive/archive_test.go`:
```go
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func entryNames(t *testing.T, data []byte) []string {
	t.Helper()
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	var names []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		names = append(names, header.Name)
	}
	return names
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func TestTarGzIncludesRegularFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name": "demo"}`)
	writeFile(t, filepath.Join(dir, "src", "index.js"), "console.log(1)")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "package.json") {
		t.Errorf("missing package.json in %v", names)
	}
	if !contains(names, "src/index.js") {
		t.Errorf("missing src/index.js in %v", names)
	}
}

func TestTarGzAlwaysExcludesGitDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	writeFile(t, filepath.Join(dir, ".git", "HEAD"), "ref: refs/heads/main")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	for _, n := range names {
		if n == ".git/HEAD" || n == ".git/" {
			t.Errorf(".git contents leaked into archive: %v", names)
		}
	}
}

func TestTarGzExcludesGitignoredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), "{}")
	writeFile(t, filepath.Join(dir, "node_modules", "leftpad", "index.js"), "module.exports = {}")
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "package.json") {
		t.Errorf("missing package.json in %v", names)
	}
	if !contains(names, ".gitignore") {
		t.Errorf("missing .gitignore itself in %v", names)
	}
	for _, n := range names {
		if n == "node_modules/leftpad/index.js" {
			t.Errorf("node_modules leaked into archive despite .gitignore: %v", names)
		}
	}
}

func TestTarGzWithNoGitignoreIncludesEverythingExceptGit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	writeFile(t, filepath.Join(dir, "node_modules", "x.js"), "x")

	var buf bytes.Buffer
	if err := TarGz(dir, &buf); err != nil {
		t.Fatalf("TarGz: %v", err)
	}

	names := entryNames(t, buf.Bytes())
	if !contains(names, "node_modules/x.js") {
		t.Errorf("expected node_modules/x.js to be included with no .gitignore present: %v", names)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/archive/... -v`
Expected: FAIL to compile — `internal/archive/archive.go` doesn't exist yet.

- [ ] **Step 4: Implement**

`internal/archive/archive.go`:
```go
package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// TarGz archives dir into a gzip-compressed tarball written to w, always
// excluding .git/, and excluding whatever dir's own .gitignore excludes if
// one is present.
func TarGz(dir string, w io.Writer) error {
	var matcher *gitignore.GitIgnore
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
		m, err := gitignore.CompileIgnoreFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			return err
		}
		matcher = m
	}

	gzw := gzip.NewWriter(w)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		if relSlash == ".git" || strings.HasPrefix(relSlash, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if matcher != nil && matcher.MatchesPath(relSlash) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relSlash
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/archive/... -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add internal/archive/archive.go internal/archive/archive_test.go go.mod go.sum
git commit -m "feat: add internal/archive for tar+gzip project archiving with .gitignore support"
```

---

### Task 5: `peak login` — real device-code flow

**Files:**
- Modify: `internal/cli/login.go`
- Modify: `go.mod` (new dependency)

**Interfaces:**
- Consumes: `api.Client.{StartDeviceAuth, PollDeviceAuth}` (Task 3); `config.{Load, SaveToken}` (existing/Task 2); shared style vars `orange`, `mutedText`, `white`, `green` (defined in `internal/cli/deploy.go`, same package `cli`).
- Produces: `loginCmd` (unchanged `Use`/registration in `root.go`) now performs a real login. No other task consumes this file's internals directly.

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/cli/browser@latest
go mod tidy
```

- [ ] **Step 2: Implement**

This file has no independent unit-testable logic (it's pure Bubbletea wiring over already-tested `api.Client` methods and `config` functions — see this plan's Global Constraints on Bubbletea testability), so this task goes straight to implementation, verified by a manual run against a locally-running backend (Step 3) rather than a new automated test.

`internal/cli/login.go` (full file, replaces the mock version):
```go
package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cli/browser"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

type loginModel struct {
	client   *api.Client
	start    *api.DeviceStartResponse
	deadline time.Time
	spinner  spinner.Model
	status   string // "starting" | "waiting" | "approved" | "error"
	err      error
}

type deviceStartedMsg struct{ start *api.DeviceStartResponse }
type deviceApprovedMsg struct{ resp *api.DevicePollResponse }
type deviceStillPendingMsg struct{}
type loginErrorMsg struct{ err error }

func initialLoginModel(client *api.Client) loginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(orange)
	return loginModel{client: client, spinner: s, status: "starting"}
}

func startDeviceAuthCmd(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		start, err := client.StartDeviceAuth()
		if err != nil {
			return loginErrorMsg{err}
		}
		return deviceStartedMsg{start}
	}
}

func pollOnceCmd(client *api.Client, deviceCode string, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		resp, err := client.PollDeviceAuth(deviceCode)
		if err != nil {
			return loginErrorMsg{err}
		}
		if resp.Status == "approved" {
			return deviceApprovedMsg{resp}
		}
		return deviceStillPendingMsg{}
	})
}

func (m loginModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, startDeviceAuthCmd(m.client))
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deviceStartedMsg:
		m.start = msg.start
		m.status = "waiting"
		m.deadline = time.Now().Add(time.Duration(msg.start.ExpiresIn) * time.Second)
		verifyURL := fmt.Sprintf("%s?user_code=%s", msg.start.VerificationURI, msg.start.UserCode)
		_ = browser.OpenURL(verifyURL)
		interval := time.Duration(msg.start.Interval) * time.Second
		return m, pollOnceCmd(m.client, msg.start.DeviceCode, interval)

	case deviceStillPendingMsg:
		if time.Now().After(m.deadline) {
			m.status = "error"
			m.err = fmt.Errorf("login timed out — run `peak login` again")
			return m, tea.Quit
		}
		interval := time.Duration(m.start.Interval) * time.Second
		return m, pollOnceCmd(m.client, m.start.DeviceCode, interval)

	case deviceApprovedMsg:
		m.status = "approved"
		if err := config.SaveToken(msg.resp.Token); err != nil {
			m.status = "error"
			m.err = err
		}
		return m, tea.Quit

	case loginErrorMsg:
		m.status = "error"
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m loginModel) View() string {
	tag := lipgloss.NewStyle().Foreground(orange).Bold(true)
	dim := lipgloss.NewStyle().Foreground(mutedText)
	ok := lipgloss.NewStyle().Foreground(green).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e05252")).Bold(true)

	switch m.status {
	case "starting":
		return fmt.Sprintf("\n  %s\n\n  %s %s\n\n", tag.Render("▲ peak login"), m.spinner.View(), dim.Render("Starting..."))
	case "waiting":
		url := fmt.Sprintf("%s?user_code=%s", m.start.VerificationURI, m.start.UserCode)
		return fmt.Sprintf(
			"\n  %s\n\n  %s\n  %s\n\n  %s %s\n\n",
			tag.Render("▲ peak login"),
			dim.Render("Opening your browser to confirm:"),
			lipgloss.NewStyle().Bold(true).Render(url),
			m.spinner.View(),
			dim.Render("Waiting for confirmation..."),
		)
	case "approved":
		return fmt.Sprintf("\n  %s\n\n", ok.Render("✓ Logged in"))
	case "error":
		return fmt.Sprintf("\n  %s  %s\n\n", errStyle.Render("✗ Login failed:"), m.err.Error())
	}
	return ""
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your 8848 Lab account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		client := api.NewClient(cfg.APIBaseURL, "")
		m := initialLoginModel(client)
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		if lm, ok := finalModel.(loginModel); ok && lm.status == "error" {
			return fmt.Errorf("login failed: %w", lm.err)
		}
		return nil
	},
}
```

- [ ] **Step 3: Verify manually**

This requires the backend plan (`2026-08-19-peak-cli-backend.md`, Himalaya repo) merged and running locally:
```bash
# In the Himalaya repo:
docker compose up -d postgres redis api worker
```
Then, in this repo:
```bash
go build -o peak ./cmd/peak
PEAK_API_URL_OVERRIDE_NOTE="config.Load() always uses DefaultAPIURL today (https://api.8848lab.org) — for a local manual check, temporarily edit DefaultAPIURL in pkg/config/config.go to http://localhost:8000, run this check, then revert the edit before committing (do not commit a changed DefaultAPIURL)."
./peak login
```
Expected: prints "Opening your browser to confirm..." with a `localhost:3000/cli-auth?user_code=...` URL, opens it in the default browser, and — once approved there — prints "✓ Logged in" and writes `~/.peak/token`. Revert any temporary `DefaultAPIURL` edit before the commit in Step 4.

- [ ] **Step 4: Commit**

```bash
git add internal/cli/login.go go.mod go.sum
git commit -m "feat: implement real device-code login flow for peak login"
```

---

### Task 6: `peak deploy` — real upload-based deploy

**Files:**
- Modify: `internal/cli/deploy.go`

**Interfaces:**
- Consumes: `api.Client.{ListOrganizations, CreateProject, DeployLocal, GetDeployment}` (Task 3); `archive.TarGz` (Task 4); `config.{Load, LoadProjectLink, SaveProjectLink, EnsureGitignored}` (Task 2); shared style vars from this same file (`orange`, `darkBg`, `mutedText`, `white`, `green`, `stepDone`, `stepPending`, `stepActive`, `envBadge`, `successBox`, `promptStyle`, `step` type) — these stay defined in this file exactly as today, only the steps/model/View below them change.
- Produces: `deployCmd` (unchanged `Use`/registration). No other task consumes this file's internals.

- [ ] **Step 1: Implement**

Like Task 5, this is Bubbletea wiring plus two plain helper functions (`resolveProjectLink`, `buildArchive`) over already-tested building blocks — no new automated test file, verified manually in Step 2.

`internal/cli/deploy.go` (full file, replaces the mock version):
```go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/internal/archive"
	"github.com/8848lab/peak/pkg/config"
)

// ── Styles ────────────────────────────────────────────────────────────────

var (
	orange    = lipgloss.Color("#E8572A")
	darkBg    = lipgloss.Color("#1a1a1a")
	mutedText = lipgloss.Color("#6b6b6b")
	white     = lipgloss.Color("#f0f0f0")
	green     = lipgloss.Color("#4caf50")

	stepDone    = lipgloss.NewStyle().Foreground(orange).SetString("✓")
	stepPending = lipgloss.NewStyle().Foreground(mutedText)
	stepActive  = lipgloss.NewStyle().Foreground(white)

	envBadge = lipgloss.NewStyle().
			Background(orange).
			Foreground(white).
			Padding(0, 1).
			Bold(true)

	successBox = lipgloss.NewStyle().
			Background(lipgloss.Color("#2a1a0e")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(orange).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().Foreground(orange).Bold(true)
)

// ── Steps ─────────────────────────────────────────────────────────────────

type step struct {
	label string
	done  bool
}

var deploySteps = []step{
	{label: "Archiving project"},
	{label: "Uploading"},
	{label: "Building"},
	{label: "Deploying"},
}

// ── Model ─────────────────────────────────────────────────────────────────

type deployModel struct {
	steps       []step
	current     int
	spinner     spinner.Model
	done        bool
	failed      bool
	errMsg      string
	projectURL  string
	elapsed     time.Duration
	startTime   time.Time
	environment string

	client       *api.Client
	projectID    string
	archivePath  string
	deploymentID string
}

type deploymentCreatedMsg struct{ id string }
type stepAdvanceMsg struct{}
type deployFailedMsg struct{ err error }
type deployReadyMsg struct{ url string }

func initialDeployModel(client *api.Client, projectID, archivePath, env string) deployModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(orange)

	return deployModel{
		steps:       deploySteps,
		spinner:     s,
		environment: env,
		startTime:   time.Now(),
		client:      client,
		projectID:   projectID,
		archivePath: archivePath,
	}
}

func (m deployModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, uploadArchiveCmd(m.client, m.projectID, m.archivePath))
}

func uploadArchiveCmd(client *api.Client, projectID, archivePath string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(archivePath)
		if err != nil {
			return deployFailedMsg{err}
		}
		defer f.Close()

		deployment, err := client.DeployLocal(projectID, f)
		if err != nil {
			return deployFailedMsg{err}
		}
		return deploymentCreatedMsg{deployment.ID}
	}
}

func pollDeploymentCmd(client *api.Client, deploymentID string) tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		deployment, err := client.GetDeployment(deploymentID)
		if err != nil {
			return deployFailedMsg{err}
		}
		switch deployment.Status {
		case "ready":
			url := ""
			if deployment.DeploymentURL != nil {
				url = *deployment.DeploymentURL
			}
			return deployReadyMsg{url}
		case "failed":
			logs := ""
			if deployment.Logs != nil {
				logs = *deployment.Logs
			}
			return deployFailedMsg{fmt.Errorf("build failed:\n%s", lastLines(logs, 15))}
		default:
			return stepAdvanceMsg{}
		}
	})
}

func lastLines(s string, n int) string {
	trimmed := strings.TrimRight(s, "\n")
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) <= n {
		return trimmed
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func (m deployModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deploymentCreatedMsg:
		m.deploymentID = msg.id
		m.steps[0].done = true
		m.steps[1].done = true
		m.current = 2 // "Building"
		return m, pollDeploymentCmd(m.client, m.deploymentID)

	case stepAdvanceMsg:
		// Still building — the backend doesn't expose discrete build phases,
		// so this just keeps the spinner alive on "Building" while polling.
		return m, pollDeploymentCmd(m.client, m.deploymentID)

	case deployReadyMsg:
		for i := range m.steps {
			m.steps[i].done = true
		}
		m.current = len(m.steps)
		m.elapsed = time.Since(m.startTime)
		m.done = true
		m.projectURL = msg.url
		return m, tea.Quit

	case deployFailedMsg:
		m.failed = true
		m.errMsg = msg.err.Error()
		m.elapsed = time.Since(m.startTime)
		return m, tea.Quit
	}

	return m, nil
}

func (m deployModel) View() string {
	header := fmt.Sprintf(
		"%s peak deploy          %s\n\n",
		promptStyle.Render("$"),
		envBadge.Render("● "+m.environment),
	)

	var steps string
	for i, s := range m.steps {
		switch {
		case s.done:
			steps += fmt.Sprintf("  %s  %s\n", stepDone.Render(), lipgloss.NewStyle().Foreground(mutedText).Render(s.label))
		case i == m.current && !m.done && !m.failed:
			steps += fmt.Sprintf("  %s  %s\n", m.spinner.View(), stepActive.Render(s.label))
		default:
			steps += fmt.Sprintf("     %s\n", stepPending.Render(s.label))
		}
	}

	view := header + steps

	if m.done {
		result := successBox.Render(
			fmt.Sprintf("✓  Deployment ready\n   %s     %ds",
				lipgloss.NewStyle().Foreground(mutedText).Render(m.projectURL),
				int(m.elapsed.Seconds()),
			),
		)
		view += "\n" + result + "\n"
	}

	if m.failed {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e05252"))
		view += "\n" + errStyle.Render("✗  "+m.errMsg) + "\n"
	}

	return view
}

// ── Command ───────────────────────────────────────────────────────────────

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your project to Himalaya",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, _ := cmd.Flags().GetString("env")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		projectID, err := resolveProjectLink(client, wd)
		if err != nil {
			return err
		}

		archivePath, err := buildArchive(wd)
		if err != nil {
			return fmt.Errorf("failed to archive project: %w", err)
		}
		defer os.Remove(archivePath)

		m := initialDeployModel(client, projectID, archivePath, env)
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("deploy failed: %w", err)
		}
		if fm, ok := finalModel.(deployModel); ok && fm.failed {
			return fmt.Errorf("deploy failed: %s", fm.errMsg)
		}
		return nil
	},
}

// resolveProjectLink returns the Himalaya project ID this directory deploys
// to — from .peak/project.json if it exists, or by prompting to create a
// new project (and writing the link file) if not.
func resolveProjectLink(client *api.Client, dir string) (string, error) {
	link, err := config.LoadProjectLink(dir)
	if err == nil {
		return link.ProjectID, nil
	}

	fmt.Println(lipgloss.NewStyle().Foreground(mutedText).Render("No linked project found in this directory — let's create one."))

	orgs, err := client.ListOrganizations()
	if err != nil {
		return "", fmt.Errorf("could not list organizations: %w", err)
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("no organizations found for your account")
	}

	reader := bufio.NewReader(os.Stdin)
	var org api.Organization
	if len(orgs) == 1 {
		org = orgs[0]
		fmt.Printf("Using organization: %s\n", org.Name)
	} else {
		fmt.Println("Select an organization:")
		for i, o := range orgs {
			fmt.Printf("  %d. %s\n", i+1, o.Name)
		}
		fmt.Print("> ")
		choice, _ := reader.ReadString('\n')
		idx, convErr := strconv.Atoi(strings.TrimSpace(choice))
		if convErr != nil || idx < 1 || idx > len(orgs) {
			return "", fmt.Errorf("invalid selection")
		}
		org = orgs[idx-1]
	}

	fmt.Print("Project name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(dir)
	}

	project, err := client.CreateProject(api.CreateProjectRequest{OrganizationID: org.ID, Name: name})
	if err != nil {
		return "", fmt.Errorf("could not create project: %w", err)
	}

	if err := config.SaveProjectLink(dir, &config.ProjectLink{
		ProjectID:      project.ID,
		OrganizationID: org.ID,
		Name:           project.Name,
	}); err != nil {
		return "", fmt.Errorf("could not save project link: %w", err)
	}
	if err := config.EnsureGitignored(dir); err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(mutedText).Render("warning: could not update .gitignore: " + err.Error()))
	}

	return project.ID, nil
}

func buildArchive(dir string) (string, error) {
	f, err := os.CreateTemp("", "peak-deploy-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := archive.TarGz(dir, f); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func init() {
	deployCmd.Flags().StringP("env", "e", "production", "Target environment (production, staging, preview)")
}
```

- [ ] **Step 2: Verify manually**

With the backend running locally (see Task 5, Step 3) and `~/.peak/token` already populated from a successful `peak login`:
```bash
go build -o peak ./cmd/peak
mkdir -p /tmp/peak-deploy-demo && cd /tmp/peak-deploy-demo
echo 'FROM node:22-alpine
CMD ["true"]' > Dockerfile
/path/to/peak deploy
```
Expected: prompts for an organization (if more than one) and a project name, creates `.peak/project.json`, then shows the animated step list (Archiving → Uploading → Building → Deploying) and finishes with a "Deployment ready" box showing a real `deployment_url`. Re-running `peak deploy` in the same directory should skip the org/project prompt (link file already exists) and go straight to archiving.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/deploy.go
git commit -m "feat: implement real upload-based deploy for peak deploy"
```

---

### Task 7: `peak logs` / `peak status` — real calls, shared deployment resolution

**Files:**
- Create: `internal/cli/resolve.go`
- Modify: `internal/cli/logs.go`
- Modify: `internal/cli/status.go`

**Interfaces:**
- Consumes: `api.Client.{GetDeployment, ListDeployments, GetContainerLogs}` (Task 3); `config.{Load, LoadProjectLink}` (existing/Task 2).
- Produces: `resolveDeploymentID(client *api.Client, arg string) (string, error)` — returns `arg` unchanged if non-empty, otherwise resolves to the current directory's linked project's most recent deployment.

- [ ] **Step 1: Implement `resolve.go`**

Like Tasks 5 and 6, this task's logic is a thin composition of already-tested `api.Client`/`config` calls with no independently meaningful unit to isolate (the only branch — "arg given vs. not" — is exercised end-to-end in Step 4's manual check), so it proceeds straight to implementation.

`internal/cli/resolve.go`:
```go
package cli

import (
	"fmt"
	"os"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

// resolveDeploymentID returns arg unchanged if non-empty, or — when no arg
// was given — the most recent deployment of the current directory's linked
// project (see config.LoadProjectLink).
func resolveDeploymentID(client *api.Client, arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	link, err := config.LoadProjectLink(wd)
	if err != nil {
		return "", fmt.Errorf("no deployment ID given and no linked project in this directory — run `peak deploy` first or pass a deployment ID")
	}

	deployments, err := client.ListDeployments(link.ProjectID)
	if err != nil {
		return "", fmt.Errorf("could not list deployments: %w", err)
	}
	if len(deployments) == 0 {
		return "", fmt.Errorf("no deployments found for this project yet")
	}
	return deployments[0].ID, nil
}
```

- [ ] **Step 2: Rewrite `logs.go`**

`internal/cli/logs.go` (full file):
```go
package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

var logsCmd = &cobra.Command{
	Use:   "logs [deployment-id]",
	Short: "Show logs for a deployment",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var arg string
		if len(args) > 0 {
			arg = args[0]
		}
		follow, _ := cmd.Flags().GetBool("follow")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		deploymentID, err := resolveDeploymentID(client, arg)
		if err != nil {
			return err
		}

		dim := lipgloss.NewStyle().Foreground(mutedText)
		tag := lipgloss.NewStyle().Foreground(orange).Bold(true)
		fmt.Printf("%s  logs for %s\n\n", tag.Render("▲ peak"), deploymentID)

		fetchLogs := func() (string, error) {
			deployment, err := client.GetDeployment(deploymentID)
			if err != nil {
				return "", err
			}
			logs := ""
			if deployment.Logs != nil {
				logs = *deployment.Logs
			}
			if deployment.Status == "ready" {
				if containerLogs, err := client.GetContainerLogs(deploymentID); err == nil && containerLogs != "" {
					logs = containerLogs
				}
			}
			return logs, nil
		}

		if !follow {
			logs, err := fetchLogs()
			if err != nil {
				return fmt.Errorf("could not fetch logs: %w", err)
			}
			fmt.Println(logs)
			return nil
		}

		fmt.Println(dim.Render("  (--follow enabled, press ctrl+c to stop)"))
		var lastLen int
		for {
			logs, err := fetchLogs()
			if err != nil {
				return fmt.Errorf("could not fetch logs: %w", err)
			}
			if len(logs) > lastLen {
				fmt.Print(logs[lastLen:])
				lastLen = len(logs)
			}
			time.Sleep(2 * time.Second)
		}
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}
```

- [ ] **Step 3: Rewrite `status.go`**

`internal/cli/status.go` (full file):
```go
package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

var statusCmd = &cobra.Command{
	Use:   "status [deployment-id]",
	Short: "Show deployment status and health",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var arg string
		if len(args) > 0 {
			arg = args[0]
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		deploymentID, err := resolveDeploymentID(client, arg)
		if err != nil {
			return err
		}

		deployment, err := client.GetDeployment(deploymentID)
		if err != nil {
			return fmt.Errorf("could not fetch status: %w", err)
		}

		label := lipgloss.NewStyle().Foreground(mutedText)
		value := lipgloss.NewStyle().Foreground(white).Bold(true)
		statusStyle := lipgloss.NewStyle().Bold(true)
		switch deployment.Status {
		case "ready":
			statusStyle = statusStyle.Foreground(green)
		case "failed":
			statusStyle = statusStyle.Foreground(lipgloss.Color("#e05252"))
		default:
			statusStyle = statusStyle.Foreground(orange)
		}

		url := "—"
		if deployment.DeploymentURL != nil {
			url = *deployment.DeploymentURL
		}

		fmt.Printf("\n  %s\n\n", lipgloss.NewStyle().Foreground(orange).Bold(true).Render("▲ peak status"))
		fmt.Printf("  %s  %s\n", label.Render("deployment"), value.Render(deployment.ID))
		fmt.Printf("  %s  %s\n", label.Render("status    "), statusStyle.Render("● "+deployment.Status))
		fmt.Printf("  %s  %s\n\n", label.Render("url       "), value.Render(url))

		return nil
	},
}
```

- [ ] **Step 4: Verify manually**

With a deployment already created via `peak deploy` (Task 6) in the current directory:
```bash
go build -o peak ./cmd/peak
./peak status
./peak logs
./peak logs --follow   # ctrl+c to stop
./peak status <a-real-deployment-id-from-the-dashboard>
```
Expected: `status` with no argument shows the linked project's latest deployment; with an explicit ID, shows that one. `logs` prints the build log (or container logs once `ready`); `--follow` keeps printing new output every 2s until interrupted.

- [ ] **Step 5: Build and vet the whole module**

Run: `go build ./... && go vet ./...`
Expected: no errors — confirms `logs.go`/`status.go`/`resolve.go` compile cleanly against everything from Tasks 1-6.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/resolve.go internal/cli/logs.go internal/cli/status.go
git commit -m "feat: implement real peak logs and peak status against the Himalaya API"
```

---

## After all tasks

Run the full test suite and a final build from the repo root:
```bash
go build -o peak ./cmd/peak
go vet ./...
go test ./...
```
Expected: builds cleanly, `go vet` is silent, and every test from Tasks 2-4 passes (`pkg/config`, `internal/api`, `internal/archive`).

Update `README.md`'s "Project Structure" (now accurate, `cmd/peak/main.go` exists) and "Roadmap" section (check off `peak login`, `peak deploy`, `peak logs`, `peak status` — the checkboxes currently describe SSE streaming and OAuth, which this plan replaces with polling and device-code auth respectively; edit the roadmap text to match what was actually built, not just check the boxes) as part of wrapping up this plan, since it's the first thing a new contributor or user reads.

Distribution (GoReleaser, GitHub Actions per-OS builds, `npm publish` so the existing `npm/bin/peak.js` wrapper resolves real binaries) is explicitly out of scope per the spec — this plan ends with a CLI that works correctly when built and run locally (`go build` + manual testing against a local or deployed Himalaya backend).
