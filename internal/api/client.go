package api

import (
	"fmt"
	"net/http"
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

// DeployRequest represents a deploy payload
type DeployRequest struct {
	Project     string `json:"project"`
	Environment string `json:"environment"`
	Branch      string `json:"branch"`
}

// DeployResponse is returned by the Himalaya API after a deploy
type DeployResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Deploy triggers a deployment via the Himalaya API
// TODO: implement once API is ready
func (c *Client) Deploy(req DeployRequest) (*DeployResponse, error) {
	_ = fmt.Sprintf("%s/v1/deploy", c.BaseURL)
	// POST to API with bearer token, decode response
	return nil, fmt.Errorf("not implemented yet")
}

// GetStatus fetches deployment status for a project
func (c *Client) GetStatus(project string) (string, error) {
	_ = fmt.Sprintf("%s/v1/projects/%s/status", c.BaseURL, project)
	return "", fmt.Errorf("not implemented yet")
}
