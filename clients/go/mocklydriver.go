package mocklydriver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AddMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// DeleteMock removes a mock by ID.
func (s *Server) DeleteMock(id string) error {
	resp, err := s.delete("/api/mocks/http/" + id)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeleteMock: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// Reset removes all dynamic mocks, deactivates scenarios, and clears faults.
func (s *Server) Reset() error {
	resp, err := s.post("/api/reset", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
	defer resp.Body.Close()
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("DeactivateScenario: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// SetFault configures global fault injection.
func (s *Server) SetFault(cfg FaultConfig) error {
	resp, err := s.post("/api/fault", cfg)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SetFault: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// ClearFault removes the active fault configuration.
func (s *Server) ClearFault() error {
	resp, err := s.delete("/api/fault")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ClearFault: unexpected status %d: %s", resp.StatusCode, body)
	}
	return nil
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
					if p.Delay != nil {
						sb.WriteString(fmt.Sprintf("        delay: %s\n", yamlStr(*p.Delay)))
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

// delete sends a DELETE request to the management API.
func (s *Server) delete(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, s.APIBase+path, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
