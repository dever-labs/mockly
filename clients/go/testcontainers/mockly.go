package testcontainersmockly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	mocklydriver "github.com/dever-labs/mockly/clients/go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	DefaultImage        = "ghcr.io/dever-labs/mockly:latest"
	HTTPPort            = "8090/tcp"
	APIPort             = "9091/tcp"
	ContainerConfigPath = "/config/mockly.yaml"
	defaultConfig       = `mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
`
)

// MocklyContainer wraps a running testcontainers container.
type MocklyContainer struct {
	testcontainers.Container
}

// Option is a functional option for configuring the container request.
type Option func(*testcontainers.GenericContainerRequest)

// WithImage overrides the default Docker image.
func WithImage(image string) Option {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Image = image
	}
}

// WithInlineConfig copies the given YAML config into /config/mockly.yaml in the container.
func WithInlineConfig(yaml string) Option {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Files = []testcontainers.ContainerFile{containerConfigFile(yaml)}
	}
}

// Run starts a Mockly container and returns a MocklyContainer.
func Run(ctx context.Context, opts ...Option) (*MocklyContainer, error) {
	req := defaultContainerRequest()

	for _, opt := range opts {
		opt(&req)
	}

	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("starting mockly container: %w", err)
	}

	return &MocklyContainer{Container: container}, nil
}

func defaultContainerRequest() testcontainers.GenericContainerRequest {
	return testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        DefaultImage,
			ExposedPorts: []string{HTTPPort, APIPort},
			Cmd:          []string{"start", "-c", ContainerConfigPath},
			Files:        []testcontainers.ContainerFile{containerConfigFile(defaultConfig)},
			WaitingFor: wait.ForHTTP("/api/protocols").
				WithPort(APIPort).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	}
}

func containerConfigFile(yaml string) testcontainers.ContainerFile {
	return testcontainers.ContainerFile{
		Reader:            strings.NewReader(yaml),
		ContainerFilePath: ContainerConfigPath,
		FileMode:          0o644,
	}
}

// HTTPBase returns the base URL for the HTTP mock server.
func (c *MocklyContainer) HTTPBase(ctx context.Context) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("getting mockly host: %w", err)
	}

	port, err := c.MappedPort(ctx, HTTPPort)
	if err != nil {
		return "", fmt.Errorf("getting mapped HTTP port: %w", err)
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}

// APIBase returns the base URL for the management API.
func (c *MocklyContainer) APIBase(ctx context.Context) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("getting mockly host: %w", err)
	}

	port, err := c.MappedPort(ctx, APIPort)
	if err != nil {
		return "", fmt.Errorf("getting mapped API port: %w", err)
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}

// AddMock registers a new mock with Mockly.
func (c *MocklyContainer) AddMock(ctx context.Context, mock mocklydriver.Mock) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/mocks/http", mock, http.StatusCreated, "AddMock")
}

// DeleteMock removes a mock by ID.
func (c *MocklyContainer) DeleteMock(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/mocks/http/"+id, nil, http.StatusNoContent, "DeleteMock")
}

// Reset removes all dynamic mocks, deactivates scenarios, and clears faults.
func (c *MocklyContainer) Reset(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/reset", nil, http.StatusOK, "Reset")
}

// ActivateScenario activates the scenario with the given ID.
func (c *MocklyContainer) ActivateScenario(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/scenarios/"+id+"/activate", nil, http.StatusOK, "ActivateScenario")
}

// DeactivateScenario deactivates the scenario with the given ID.
func (c *MocklyContainer) DeactivateScenario(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/scenarios/"+id+"/deactivate", nil, http.StatusOK, "DeactivateScenario")
}

// SetFault configures a direct HTTP fault.
func (c *MocklyContainer) SetFault(ctx context.Context, cfg mocklydriver.FaultConfig) error {
	payload := map[string]any{}
	if cfg.Delay != "" {
		payload["delay"] = cfg.Delay
	}
	if cfg.StatusOverride != nil {
		payload["status"] = *cfg.StatusOverride
	}
	if cfg.ErrorRate != 0 {
		payload["error_rate"] = cfg.ErrorRate
	}

	return c.expectStatus(ctx, http.MethodPost, "/api/fault/http", payload, http.StatusOK, "SetFault")
}

// ClearFault removes all direct fault configuration.
func (c *MocklyContainer) ClearFault(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/fault", nil, http.StatusOK, "ClearFault", http.StatusNoContent)
}

// GetLogs fetches recent log entries from the management API.
func (c *MocklyContainer) GetLogs(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/logs", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading GetLogs response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GetLogs: unexpected status %d: %s", resp.StatusCode, body)
	}

	return string(body), nil
}

// ClearLogs clears all stored log entries.
func (c *MocklyContainer) ClearLogs(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/logs", nil, http.StatusOK, "ClearLogs")
}

func (c *MocklyContainer) expectStatus(ctx context.Context, method, path string, body any, expected int, op string, additional ...int) error {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", op, err)
	}

	allowed := append([]int{expected}, additional...)
	for _, status := range allowed {
		if resp.StatusCode == status {
			return nil
		}
	}

	return fmt.Errorf("%s: unexpected status %d: %s", op, resp.StatusCode, responseBody)
}

func (c *MocklyContainer) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	baseURL, err := c.APIBase(ctx)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("creating %s request for %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing %s request for %s: %w", method, path, err)
	}

	return resp, nil
}
