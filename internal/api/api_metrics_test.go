package api_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/metrics"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/natsserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// GET /metrics — opt-in Prometheus endpoint (issue #215)
// ---------------------------------------------------------------------------

func newAPIServerForMetricsTest(t *testing.T, cfg *config.Config, httpStub *stubHTTP) (string, *api.Server) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cfg.Mockly.API.Port = port

	srv := api.New(
		cfg, state.New(), scenarios.New(nil), logger.New(100), nil,
		httpStub,
		&stubWS{},
		&stubGRPC{},
		&stubGraphQL{},
		&stubTCP{},
		&stubRedis{},
		&stubSMTP{inbox: smtpserver.NewInbox(50)},
		&stubMQTT{ms: mqttserver.NewMessageStore(50)},
		&stubNATS{ms: natsserver.NewMessageStore(50)},
		&stubSNMP{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base+"/api/protocols", 2*time.Second)
	return base, srv
}

func TestAPI_Metrics_NotRegisteredWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	base, srv := newAPIServerForMetricsTest(t, cfg, &stubHTTP{})
	srv.SetMetrics(metrics.New(nil)) // attached, but config.Metrics is nil → route not registered

	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when metrics disabled, got %d", resp.StatusCode)
	}
}

func TestAPI_Metrics_NotRegisteredWithoutSetMetrics(t *testing.T) {
	cfg := &config.Config{}
	cfg.Mockly.API.Metrics = &config.MetricsConfig{Enabled: true}
	base, _ := newAPIServerForMetricsTest(t, cfg, &stubHTTP{})

	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when SetMetrics was never called, got %d", resp.StatusCode)
	}
}

func TestAPI_Metrics_ExposedWhenEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Mockly.API.Metrics = &config.MetricsConfig{Enabled: true}
	httpStub := &stubHTTP{mocks: []config.HTTPMock{{ID: "a"}, {ID: "b"}}}
	base, srv := newAPIServerForMetricsTest(t, cfg, httpStub)

	reg := metrics.New(func() float64 { return float64(len(httpStub.GetMocks())) })
	reg.ObserveHTTPRequest("a", "GET", 200, 0.01)
	srv.SetMetrics(reg)

	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	for _, want := range []string{
		`mockly_http_requests_total{method="GET",mock_id="a",status="200"} 1`,
		"mockly_active_mocks 2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("scrape missing %q, got:\n%s", want, got)
		}
	}
}
