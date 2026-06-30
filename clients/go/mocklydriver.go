package mocklydriver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Server represents a running Mockly process.
type Server struct {
	HTTPPort int    // mock HTTP server port
	APIPort  int    // management API port
	HTTPBase string // "http://127.0.0.1:<HTTPPort>"
	APIBase  string // "http://127.0.0.1:<APIPort>"
	proc     *exec.Cmd
	stderr   *bytes.Buffer
}

// Create finds the Mockly binary via GetBinaryPath, allocates ports, starts the process,
// and retries up to 3× on port conflict.
func Create(opts Options) (*Server, error) {
	binPath := GetBinaryPath("")
	if binPath == "" {
		return nil, fmt.Errorf(
			"mockly binary not found; run mocklydriver.Install() or set MOCKLY_BINARY_PATH",
		)
	}

	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		httpPort, err := getFreePort()
		if err != nil {
			return nil, fmt.Errorf("allocating HTTP port: %w", err)
		}
		apiPort, err := getFreePort()
		if err != nil {
			return nil, fmt.Errorf("allocating API port: %w", err)
		}

		s := &Server{
			HTTPPort: httpPort,
			APIPort:  apiPort,
			HTTPBase: fmt.Sprintf("http://127.0.0.1:%d", httpPort),
			APIBase:  fmt.Sprintf("http://127.0.0.1:%d", apiPort),
			stderr:   &bytes.Buffer{},
		}

		if err := s.start(binPath, opts.Scenarios); err != nil {
			if isPortConflict(err.Error()) && attempt < maxAttempts-1 {
				continue
			}
			return nil, err
		}
		return s, nil
	}
	return nil, fmt.Errorf("failed to start mockly after %d attempts (port conflict)", maxAttempts)
}

// Ensure runs Install() and then Create().
func Ensure(opts Options, installOpts InstallOptions) (*Server, error) {
	if _, err := Install(installOpts); err != nil {
		return nil, fmt.Errorf("installing mockly: %w", err)
	}
	return Create(opts)
}

// Stop kills the Mockly process and waits for it to exit.
func (s *Server) Stop() error {
	if s.proc == nil || s.proc.Process == nil {
		return nil
	}
	if err := s.proc.Process.Kill(); err != nil {
		return fmt.Errorf("killing mockly process: %w", err)
	}
	_ = s.proc.Wait()
	return nil
}

// AddMock registers a new mock with Mockly.
func (s *Server) AddMock(mock Mock) error {
	resp, err := s.post("/api/mocks/http", mock)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AddMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ListMocks returns all configured HTTP mocks.
func (s *Server) ListMocks() ([]Mock, error) {
	resp, err := s.get("/api/mocks/http")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListMocks: unexpected status %d: %s", resp.StatusCode, body)
	}
	var mocks []Mock
	if err := json.NewDecoder(resp.Body).Decode(&mocks); err != nil {
		return nil, fmt.Errorf("ListMocks: decoding response: %w", err)
	}
	return mocks, nil
}

// UpdateMock replaces an existing mock and returns the updated value.
func (s *Server) UpdateMock(id string, mock Mock) (*Mock, error) {
	resp, err := s.put("/api/mocks/http/"+url.PathEscape(id), mock)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("UpdateMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	var updated Mock
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("UpdateMock: decoding response: %w", err)
	}
	return &updated, nil
}

// PatchMock applies a partial response update and returns the updated mock.
func (s *Server) PatchMock(id string, patch MockResponsePatch) (*Mock, error) {
	resp, err := s.patch("/api/mocks/http/"+url.PathEscape(id), patch)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PatchMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	var updated Mock
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("PatchMock: decoding response: %w", err)
	}
	return &updated, nil
}

// DeleteMock removes a mock by ID.
func (s *Server) DeleteMock(id string) error {
	resp, err := s.delete("/api/mocks/http/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetState returns the full server state map.
func (s *Server) GetState() (map[string]string, error) {
	resp, err := s.get("/api/state")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetState: unexpected status %d: %s", resp.StatusCode, body)
	}
	var state map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, fmt.Errorf("GetState: decoding response: %w", err)
	}
	return state, nil
}

// SetState updates server state and returns the resulting map.
func (s *Server) SetState(state map[string]string) (map[string]string, error) {
	resp, err := s.post("/api/state", state)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SetState: unexpected status %d: %s", resp.StatusCode, body)
	}
	var updated map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("SetState: decoding response: %w", err)
	}
	return updated, nil
}

// DeleteState removes a single state key.
func (s *Server) DeleteState(key string) error {
	resp, err := s.delete("/api/state/" + url.PathEscape(key))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteState: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetLogs returns recorded logs, optionally filtered by matched mock ID.
func (s *Server) GetLogs(matchedID string) ([]CallEntry, error) {
	path := "/api/logs"
	if matchedID != "" {
		q := url.Values{}
		q.Set("matched_id", matchedID)
		path += "?" + q.Encode()
	}
	resp, err := s.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetLogs: unexpected status %d: %s", resp.StatusCode, body)
	}
	var entries []CallEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("GetLogs: decoding response: %w", err)
	}
	return entries, nil
}

// ClearLogs clears all recorded logs.
func (s *Server) ClearLogs() error {
	resp, err := s.delete("/api/logs")
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ClearLogs: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetLogsCount returns the number of recorded logs, optionally filtered by matched mock ID.
func (s *Server) GetLogsCount(matchedID string) (int, error) {
	path := "/api/logs/count"
	if matchedID != "" {
		q := url.Values{}
		q.Set("matched_id", matchedID)
		path += "?" + q.Encode()
	}
	resp, err := s.get(path)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GetLogsCount: unexpected status %d: %s", resp.StatusCode, body)
	}
	var count struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return 0, fmt.Errorf("GetLogsCount: decoding response: %w", err)
	}
	return count.Count, nil
}

// ListScenarios returns all configured scenarios.
func (s *Server) ListScenarios() ([]Scenario, error) {
	resp, err := s.get("/api/scenarios")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListScenarios: unexpected status %d: %s", resp.StatusCode, body)
	}
	var scenarios []Scenario
	if err := json.NewDecoder(resp.Body).Decode(&scenarios); err != nil {
		return nil, fmt.Errorf("ListScenarios: decoding response: %w", err)
	}
	return scenarios, nil
}

// CreateScenario registers a scenario and returns the created value.
func (s *Server) CreateScenario(scenario Scenario) (*Scenario, error) {
	resp, err := s.post("/api/scenarios", scenario)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CreateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	var created Scenario
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("CreateScenario: decoding response: %w", err)
	}
	return &created, nil
}

// GetScenario returns a scenario by ID.
func (s *Server) GetScenario(id string) (*Scenario, error) {
	resp, err := s.get("/api/scenarios/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	var scenario Scenario
	if err := json.NewDecoder(resp.Body).Decode(&scenario); err != nil {
		return nil, fmt.Errorf("GetScenario: decoding response: %w", err)
	}
	return &scenario, nil
}

// UpdateScenario replaces a scenario and returns the updated value.
func (s *Server) UpdateScenario(id string, scenario Scenario) (*Scenario, error) {
	resp, err := s.put("/api/scenarios/"+url.PathEscape(id), scenario)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("UpdateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	var updated Scenario
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		return nil, fmt.Errorf("UpdateScenario: decoding response: %w", err)
	}
	return &updated, nil
}

// DeleteScenario removes a scenario by ID.
func (s *Server) DeleteScenario(id string) error {
	resp, err := s.delete("/api/scenarios/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ListActiveScenarios returns active scenario IDs together with their full definitions.
func (s *Server) ListActiveScenarios() (*ActiveScenariosResponse, error) {
	resp, err := s.get("/api/scenarios/active")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ListActiveScenarios: unexpected status %d: %s", resp.StatusCode, body)
	}
	var active ActiveScenariosResponse
	if err := json.NewDecoder(resp.Body).Decode(&active); err != nil {
		return nil, fmt.Errorf("ListActiveScenarios: decoding response: %w", err)
	}
	return &active, nil
}

// Reset removes all dynamic mocks, deactivates scenarios, and clears faults.
func (s *Server) Reset() error {
	resp, err := s.post("/api/reset", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Reset: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ActivateScenario activates the scenario with the given ID.
func (s *Server) ActivateScenario(id string) error {
	resp, err := s.post("/api/scenarios/"+id+"/activate", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ActivateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// DeactivateScenario deactivates the scenario with the given ID.
func (s *Server) DeactivateScenario(id string) error {
	resp, err := s.post("/api/scenarios/"+id+"/deactivate", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeactivateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// SetFault configures a direct HTTP fault.
func (s *Server) SetFault(cfg FaultConfig) error {
	payload := map[string]interface{}{}
	if cfg.Delay != "" {
		payload["delay"] = cfg.Delay
	}
	if cfg.StatusOverride != nil {
		payload["status"] = *cfg.StatusOverride
	}
	if cfg.ErrorRate != 0 {
		payload["error_rate"] = cfg.ErrorRate
	}
	resp, err := s.post("/api/fault/http", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SetFault: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ClearFault removes all direct fault configuration.
func (s *Server) ClearFault() error {
	resp, err := s.delete("/api/fault")
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ClearFault: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetCalls returns recorded calls for the given mock ID.
func (s *Server) GetCalls(mockID string) (*CallSummary, error) {
	resp, err := s.get("/api/calls/http/" + mockID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetCalls: unexpected status %d: %s", resp.StatusCode, body)
	}
	var summary CallSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("GetCalls: decoding response: %w", err)
	}
	return &summary, nil
}

// ClearCalls clears recorded calls for the given mock ID.
func (s *Server) ClearCalls(mockID string) error {
	resp, err := s.delete("/api/calls/http/" + mockID)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ClearCalls: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ClearAllCalls clears recorded calls for all mocks.
func (s *Server) ClearAllCalls() error {
	resp, err := s.delete("/api/calls/http")
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ClearAllCalls: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// WaitForCalls blocks until mockID has been called at least count times,
// or until timeout expires. Returns the recorded calls on success.
func (s *Server) WaitForCalls(mockID string, count int, timeout time.Duration) (*CallSummary, error) {
	payload := map[string]interface{}{
		"count":   count,
		"timeout": timeout.String(),
	}
	resp, err := s.post("/api/calls/http/"+mockID+"/wait", payload)
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
	var summary CallSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("WaitForCalls: decoding response: %w", err)
	}
	return &summary, nil
}

// start writes a config file, spawns the process, and waits for readiness.
func (s *Server) start(binPath string, scenarios []Scenario) error {
	configPath, err := s.writeConfig(scenarios)
	if err != nil {
		return fmt.Errorf("writing mockly config: %w", err)
	}

	cmd := exec.Command(binPath, "start",
		"--config", configPath,
		"--api-port", fmt.Sprintf("%d", s.APIPort),
	)
	cmd.Stderr = s.stderr
	s.proc = cmd

	if err := cmd.Start(); err != nil {
		os.Remove(configPath)
		return fmt.Errorf("starting mockly: %w", err)
	}

	if err := s.waitReady(15 * time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		os.Remove(configPath)
		stderrOut := s.stderr.String()
		if isPortConflict(stderrOut) {
			return fmt.Errorf("port conflict: %s", stderrOut)
		}
		return fmt.Errorf("mockly did not become ready: %w (stderr: %s)", err, stderrOut)
	}

	// Config file is consumed; clean up.
	os.Remove(configPath)
	return nil
}

// writeConfig writes a YAML config file for Mockly and returns the file path.
func (s *Server) writeConfig(scenarios []Scenario) (string, error) {
	var sb strings.Builder

	sb.WriteString("mockly:\n")
	sb.WriteString("  api:\n")
	sb.WriteString(fmt.Sprintf("    port: %d\n", s.APIPort))
	sb.WriteString("protocols:\n")
	sb.WriteString("  http:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString(fmt.Sprintf("    port: %d\n", s.HTTPPort))

	if len(scenarios) > 0 {
		sb.WriteString("scenarios:\n")
		for _, sc := range scenarios {
			sb.WriteString(fmt.Sprintf("  - id: %s\n", yamlStr(sc.ID)))
			sb.WriteString(fmt.Sprintf("    name: %s\n", yamlStr(sc.Name)))
			if sc.Description != "" {
				sb.WriteString(fmt.Sprintf("    description: %s\n", yamlStr(sc.Description)))
			}
			if len(sc.Patches) > 0 {
				sb.WriteString("    patches:\n")
				for _, p := range sc.Patches {
					sb.WriteString(fmt.Sprintf("      - mock_id: %s\n", yamlStr(p.MockID)))
					if p.Status != nil {
						sb.WriteString(fmt.Sprintf("        status: %d\n", *p.Status))
					}
					if p.Body != nil {
						sb.WriteString(fmt.Sprintf("        body: %s\n", yamlStr(*p.Body)))
					}
					if len(p.Headers) > 0 {
						sb.WriteString("        headers:\n")
						for key, value := range p.Headers {
							sb.WriteString(fmt.Sprintf("          %s: %s\n", yamlStr(key), yamlStr(value)))
						}
					}
					if p.Delay != nil {
						sb.WriteString(fmt.Sprintf("        delay: %s\n", yamlStr(*p.Delay)))
					}
					if p.Disabled != nil {
						sb.WriteString(fmt.Sprintf("        disabled: %t\n", *p.Disabled))
					}
				}
			}
		}
	}

	f, err := os.CreateTemp("", "mockly-config-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(sb.String()); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// waitReady polls GET /api/protocols until Mockly responds or the deadline is reached.
func (s *Server) waitReady(maxDuration time.Duration) error {
	deadline := time.Now().Add(maxDuration)
	url := s.APIBase + "/api/protocols"
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url) //nolint:gosec
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for mockly to be ready after %s", maxDuration)
}

// get sends a GET request to the management API.
func (s *Server) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, s.APIBase+path, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// post sends a POST request to the management API.
func (s *Server) post(path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		r = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPost, s.APIBase+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

// put sends a PUT request to the management API.
func (s *Server) put(path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		r = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPut, s.APIBase+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

// patch sends a PATCH request to the management API.
func (s *Server) patch(path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		r = bytes.NewReader(data)
	}

	req, err := http.NewRequest(http.MethodPatch, s.APIBase+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

// delete sends a DELETE request to the management API.
func (s *Server) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, s.APIBase+path, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
