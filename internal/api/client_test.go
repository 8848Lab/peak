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

func TestPollDeviceAuthSendsDeviceCodeAsJSONBody(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/device/poll" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			DeviceCode string `json:"device_code"`
		}
		json.Unmarshal(body, &req)
		if req.DeviceCode != "dc123" {
			t.Errorf("missing/wrong device_code in body: %s", string(body))
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
