package mocklydriver

// CallEntry is a single HTTP request recorded by Mockly.
type CallEntry struct {
	ID         string            `json:"id"`
	Timestamp  string            `json:"timestamp"`
	Protocol   string            `json:"protocol"`
	Method     string            `json:"method,omitempty"`
	Path       string            `json:"path"`
	Status     int               `json:"status,omitempty"`
	DurationMs int64             `json:"duration_ms"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	MatchedID  string            `json:"matched_id,omitempty"`
	PathParams map[string]string `json:"path_params,omitempty"`
}

// CallSummary holds recorded calls for a specific HTTP mock.
type CallSummary struct {
	MockID string      `json:"mock_id"`
	Count  int64       `json:"count"`
	Calls  []CallEntry `json:"calls"`
}

// MockRequest describes the conditions a request must match.
type MockRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

// MockResponse describes the response Mockly will return.
type MockResponse struct {
	Status  int               `json:"status"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Delay   string            `json:"delay,omitempty"` // e.g. "50ms"
}

// MockResponsePatch describes a partial response update for an existing mock.
type MockResponsePatch struct {
	Status  *int              `json:"status,omitempty"`
	Body    *string           `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Delay   *string           `json:"delay,omitempty"`
}

// Mock is a request/response pair registered with Mockly.
type Mock struct {
	ID       string       `json:"id"`
	Request  MockRequest  `json:"request"`
	Response MockResponse `json:"response"`
}

// ScenarioPatch overrides a mock's behaviour when a scenario is active.
type ScenarioPatch struct {
	MockID   string            `json:"mock_id"`
	Status   *int              `json:"status,omitempty"`
	Body     *string           `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Delay    *string           `json:"delay,omitempty"`
	Disabled *bool             `json:"disabled,omitempty"`
}

// Scenario groups patches that are applied together when activated.
type Scenario struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Patches     []ScenarioPatch `json:"patches"`
}

// ActiveScenariosResponse reports active scenario IDs and their full definitions.
type ActiveScenariosResponse struct {
	Active    []string   `json:"active"`
	Scenarios []Scenario `json:"scenarios"`
}

// FaultConfig configures global fault injection.
type FaultConfig struct {
	Enabled        bool    `json:"enabled"`
	Delay          string  `json:"delay,omitempty"`
	Status    *int    `json:"status,omitempty"`
	ErrorRate      float64 `json:"error_rate,omitempty"` // 0.0–1.0
}

// Options controls MocklyServer startup.
type Options struct {
	Scenarios []Scenario
}

// InstallOptions controls binary download behaviour.
type InstallOptions struct {
	Version string // default: DefaultMocklyVersion
	BaseURL string // default: GitHub releases
	BinDir  string // default: ./bin
	Force   bool   // re-download even if binary exists
}
