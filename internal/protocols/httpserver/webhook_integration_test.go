package httpserver_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/httpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/webhook"
)

// startTestServerWithWebhooks is like startTestServer but wires a real
// *webhook.Sender, letting tests verify that a matched mock's Webhooks fire
// an outbound HTTP call.
func startTestServerWithWebhooks(t *testing.T, mocks []config.HTTPMock, wh *webhook.Sender) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.HTTPConfig{Enabled: true, Port: port, Mocks: mocks}
	srv := httpserver.New(cfg, state.New(), scenarios.New(nil), logger.New(100), wh)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHTTP(t, base, 2*time.Second)
	return base
}

func TestHTTPServer_MockFiresWebhookOnMatch(t *testing.T) {
	received := make(chan string, 1)
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		received <- string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer callback.Close()

	mocks := []config.HTTPMock{{
		ID:      "create-payment",
		Request: config.HTTPRequest{Method: "POST", Path: "/v1/payment"},
		Response: config.HTTPResponse{
			Status: 201,
			Body:   `{"paymentId":"pay_1"}`,
		},
		Webhooks: []config.Webhook{{
			URL:    callback.URL + "/cb",
			Method: "POST",
			Body:   `{"event":"payment.created","id":"{{.request.body.id}}"}`,
		}},
	}}

	wh := webhook.New(10)
	base := startTestServerWithWebhooks(t, mocks, wh)

	resp, err := http.Post(base+"/v1/payment", "application/json", strings.NewReader(`{"id":"order-42"}`))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	select {
	case body := <-received:
		if body != `{"event":"payment.created","id":"order-42"}` {
			t.Errorf("webhook body = %q, unexpected", body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected webhook was not received")
	}

	// Give the sender a moment to record the attempt after responding.
	time.Sleep(50 * time.Millisecond)
	records := wh.History().All()
	if len(records) != 1 {
		t.Fatalf("history len = %d, want 1", len(records))
	}
	if records[0].MockID != "create-payment" {
		t.Errorf("mock ID = %q, want create-payment", records[0].MockID)
	}
}
