package testcontainersmockly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// ListMocks returns all configured HTTP mocks.
func (c *MocklyContainer) ListMocks(ctx context.Context) ([]mocklydriver.Mock, error) {
	resp, err := c.get(ctx, "/api/mocks/http")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListMocks: unexpected status %d: %s", resp.StatusCode, body)
	}

	mocks, err := decodeResponse[[]mocklydriver.Mock](resp, "ListMocks")
	if err != nil {
		return nil, err
	}

	return mocks, nil
}

// UpdateMock replaces an existing mock and returns the updated value.
func (c *MocklyContainer) UpdateMock(ctx context.Context, id string, mock mocklydriver.Mock) (*mocklydriver.Mock, error) {
	resp, err := c.put(ctx, "/api/mocks/http/"+url.PathEscape(id), mock)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("UpdateMock: unexpected status %d: %s", resp.StatusCode, body)
	}

	updated, err := decodeResponse[mocklydriver.Mock](resp, "UpdateMock")
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

// PatchMock applies a partial response update and returns the updated mock.
func (c *MocklyContainer) PatchMock(ctx context.Context, id string, patch mocklydriver.MockResponsePatch) (*mocklydriver.Mock, error) {
	resp, err := c.patch(ctx, "/api/mocks/http/"+url.PathEscape(id), patch)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PatchMock: unexpected status %d: %s", resp.StatusCode, body)
	}

	updated, err := decodeResponse[mocklydriver.Mock](resp, "PatchMock")
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

// DeleteMock removes a mock by ID.
func (c *MocklyContainer) DeleteMock(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/mocks/http/"+url.PathEscape(id), nil, http.StatusOK, "DeleteMock")
}

// GetState returns the full server state map.
func (c *MocklyContainer) GetState(ctx context.Context) (map[string]string, error) {
	resp, err := c.get(ctx, "/api/state")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetState: unexpected status %d: %s", resp.StatusCode, body)
	}

	state, err := decodeResponse[map[string]string](resp, "GetState")
	if err != nil {
		return nil, err
	}

	return state, nil
}

// SetState updates server state and returns the resulting map.
func (c *MocklyContainer) SetState(ctx context.Context, state map[string]string) (map[string]string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/state", state)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SetState: unexpected status %d: %s", resp.StatusCode, body)
	}

	updated, err := decodeResponse[map[string]string](resp, "SetState")
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// DeleteState removes a single state key.
func (c *MocklyContainer) DeleteState(ctx context.Context, key string) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/state/"+url.PathEscape(key), nil, http.StatusOK, "DeleteState")
}

// Reset removes all dynamic mocks, deactivates scenarios, and clears faults.
func (c *MocklyContainer) Reset(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/reset", nil, http.StatusOK, "Reset")
}

// ActivateScenario activates the scenario with the given ID.
func (c *MocklyContainer) ActivateScenario(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/scenarios/"+url.PathEscape(id)+"/activate", nil, http.StatusOK, "ActivateScenario")
}

// DeactivateScenario deactivates the scenario with the given ID.
func (c *MocklyContainer) DeactivateScenario(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodPost, "/api/scenarios/"+url.PathEscape(id)+"/deactivate", nil, http.StatusOK, "DeactivateScenario")
}

// ListScenarios returns all configured scenarios.
func (c *MocklyContainer) ListScenarios(ctx context.Context) ([]mocklydriver.Scenario, error) {
	resp, err := c.get(ctx, "/api/scenarios")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListScenarios: unexpected status %d: %s", resp.StatusCode, body)
	}

	scenarios, err := decodeResponse[[]mocklydriver.Scenario](resp, "ListScenarios")
	if err != nil {
		return nil, err
	}

	return scenarios, nil
}

// CreateScenario registers a scenario and returns the created value.
func (c *MocklyContainer) CreateScenario(ctx context.Context, scenario mocklydriver.Scenario) (*mocklydriver.Scenario, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/scenarios", scenario)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CreateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}

	created, err := decodeResponse[mocklydriver.Scenario](resp, "CreateScenario")
	if err != nil {
		return nil, err
	}

	return &created, nil
}

// GetScenario returns a scenario by ID.
func (c *MocklyContainer) GetScenario(ctx context.Context, id string) (*mocklydriver.Scenario, error) {
	resp, err := c.get(ctx, "/api/scenarios/"+url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetScenario: unexpected status %d: %s", resp.StatusCode, body)
	}

	scenario, err := decodeResponse[mocklydriver.Scenario](resp, "GetScenario")
	if err != nil {
		return nil, err
	}

	return &scenario, nil
}

// UpdateScenario replaces a scenario and returns the updated value.
func (c *MocklyContainer) UpdateScenario(ctx context.Context, id string, scenario mocklydriver.Scenario) (*mocklydriver.Scenario, error) {
	resp, err := c.put(ctx, "/api/scenarios/"+url.PathEscape(id), scenario)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("UpdateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}

	updated, err := decodeResponse[mocklydriver.Scenario](resp, "UpdateScenario")
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

// DeleteScenario removes a scenario by ID.
func (c *MocklyContainer) DeleteScenario(ctx context.Context, id string) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/scenarios/"+url.PathEscape(id), nil, http.StatusOK, "DeleteScenario")
}

// ListActiveScenarios returns active scenario IDs together with their full definitions.
func (c *MocklyContainer) ListActiveScenarios(ctx context.Context) (*mocklydriver.ActiveScenariosResponse, error) {
	resp, err := c.get(ctx, "/api/scenarios/active")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListActiveScenarios: unexpected status %d: %s", resp.StatusCode, body)
	}

	active, err := decodeResponse[mocklydriver.ActiveScenariosResponse](resp, "ListActiveScenarios")
	if err != nil {
		return nil, err
	}

	return &active, nil
}

// SetFault configures a direct HTTP fault.
func (c *MocklyContainer) SetFault(ctx context.Context, cfg mocklydriver.FaultConfig) error {
	payload := map[string]any{}
	if cfg.Delay != "" {
		payload["delay"] = cfg.Delay
	}
	if cfg.Status != nil {
		payload["status"] = *cfg.Status
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
func (c *MocklyContainer) GetLogs(ctx context.Context, matchedID string) ([]mocklydriver.CallEntry, error) {
	path := "/api/logs"
	if matchedID != "" {
		q := url.Values{}
		q.Set("matched_id", matchedID)
		path += "?" + q.Encode()
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetLogs: unexpected status %d: %s", resp.StatusCode, body)
	}

	logs, err := decodeResponse[[]mocklydriver.CallEntry](resp, "GetLogs")
	if err != nil {
		return nil, err
	}

	return logs, nil
}

// ClearLogs clears all stored log entries.
func (c *MocklyContainer) ClearLogs(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/logs", nil, http.StatusOK, "ClearLogs")
}

// GetLogsCount returns the number of recorded logs, optionally filtered by matched mock ID.
func (c *MocklyContainer) GetLogsCount(ctx context.Context, matchedID string) (int, error) {
	path := "/api/logs/count"
	if matchedID != "" {
		q := url.Values{}
		q.Set("matched_id", matchedID)
		path += "?" + q.Encode()
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GetLogsCount: unexpected status %d: %s", resp.StatusCode, body)
	}

	count, err := decodeResponse[struct {
		Count int `json:"count"`
	}](resp, "GetLogsCount")
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

// GetCalls returns recorded calls for the given mock ID.
func (c *MocklyContainer) GetCalls(ctx context.Context, mockID string) (*mocklydriver.CallSummary, error) {
	resp, err := c.get(ctx, "/api/calls/http/"+url.PathEscape(mockID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetCalls: unexpected status %d: %s", resp.StatusCode, body)
	}

	summary, err := decodeResponse[mocklydriver.CallSummary](resp, "GetCalls")
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

// ClearCalls clears recorded calls for the given mock ID.
func (c *MocklyContainer) ClearCalls(ctx context.Context, mockID string) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/calls/http/"+url.PathEscape(mockID), nil, http.StatusOK, "ClearCalls")
}

// ClearAllCalls clears recorded calls for all mocks.
func (c *MocklyContainer) ClearAllCalls(ctx context.Context) error {
	return c.expectStatus(ctx, http.MethodDelete, "/api/calls/http", nil, http.StatusOK, "ClearAllCalls")
}

// WaitForCalls blocks until mockID has been called at least count times.
func (c *MocklyContainer) WaitForCalls(ctx context.Context, mockID string, count int, timeout time.Duration) (*mocklydriver.CallSummary, error) {
	payload := map[string]any{
		"count":   count,
		"timeout": timeout.String(),
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/calls/http/"+url.PathEscape(mockID)+"/wait", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusRequestTimeout {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WaitForCalls: timeout waiting for %d call(s) on %q: %s", count, mockID, body)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WaitForCalls: unexpected status %d: %s", resp.StatusCode, body)
	}

	summary, err := decodeResponse[mocklydriver.CallSummary](resp, "WaitForCalls")
	if err != nil {
		return nil, err
	}

	return &summary, nil
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

func decodeResponse[T any](resp *http.Response, op string) (T, error) {
	var decoded T
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return decoded, fmt.Errorf("%s: decoding response: %w", op, err)
	}
	return decoded, nil
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

func (c *MocklyContainer) get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *MocklyContainer) put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPut, path, body)
}

func (c *MocklyContainer) patch(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodPatch, path, body)
}
