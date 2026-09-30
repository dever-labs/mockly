package metrics_test

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/metrics"
)

func TestRegistry_ObserveHTTPRequest_ExposesCounterAndHistogram(t *testing.T) {
	reg := metrics.New(func() float64 { return 3 })
	reg.ObserveHTTPRequest("get-user", "GET", 200, 0.015)
	reg.ObserveHTTPRequest("get-user", "GET", 200, 0.02)
	reg.ObserveHTTPRequest("", "POST", 404, 0.001) // unmatched

	body := scrape(t, reg)

	wantSubstrings := []string{
		`mockly_http_requests_total{method="GET",mock_id="get-user",status="200"} 2`,
		`mockly_http_requests_total{method="POST",mock_id="unmatched",status="404"} 1`,
		`mockly_http_request_duration_seconds_count{mock_id="get-user"} 2`,
		`mockly_active_mocks 3`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("scrape output missing %q\n--- full output ---\n%s", want, body)
		}
	}
}

func TestRegistry_ActiveMocksGauge_ReflectsCallbackAtScrapeTime(t *testing.T) {
	count := 0
	reg := metrics.New(func() float64 { return float64(count) })

	body := scrape(t, reg)
	if !strings.Contains(body, "mockly_active_mocks 0") {
		t.Errorf("expected mockly_active_mocks 0, got:\n%s", body)
	}

	count = 5
	body = scrape(t, reg)
	if !strings.Contains(body, "mockly_active_mocks 5") {
		t.Errorf("expected mockly_active_mocks 5 after update, got:\n%s", body)
	}
}

func TestRegistry_New_NilActiveMocksFuncDefaultsToZero(t *testing.T) {
	reg := metrics.New(nil)
	body := scrape(t, reg)
	if !strings.Contains(body, "mockly_active_mocks 0") {
		t.Errorf("expected mockly_active_mocks 0 with nil callback, got:\n%s", body)
	}
}

func scrape(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	reg.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("scrape: want 200, got %d", rr.Code)
	}
	b, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("read scrape body: %v", err)
	}
	return string(b)
}
