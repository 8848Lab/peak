package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client is the HTTP client for the Himalaya API
type Client struct {
	BaseURL      string
	Token        string
	httpClient   *http.Client
	uploadClient *http.Client
}

// NewClient creates an authenticated API client
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		// Uploads (e.g. DeployLocal) can carry archives up to 250MB — the
		// standard 30s client-wide timeout would abort large transfers
		// mid-flight on an ordinary connection, so give uploads a much
		// longer budget instead of reusing httpClient.
		uploadClient: &http.Client{
			Timeout: 10 * time.Minute,
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
	body := struct {
		DeviceCode string `json:"device_code"`
	}{DeviceCode: deviceCode}
	if err := c.doJSON(http.MethodPost, "/auth/device/poll", body, &out); err != nil {
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

	resp, err := c.uploadClient.Do(req)
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
