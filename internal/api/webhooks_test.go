package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/api"
	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/mqttserver"
	"github.com/dever-labs/mockly/internal/protocols/natsserver"
	"github.com/dever-labs/mockly/internal/protocols/smtpserver"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/webhook"
)

// startAPIWithWebhooks starts the API with a real *webhook.Sender wired in,
// so /api/webhooks* endpoints can be exercised end-to-end.
func startAPIWithWebhooks(t *testing.T, wh *webhook.Sender) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{}
	cfg.Mockly.API.Port = port

	srv := api.New(
		cfg, state.New(), scenarios.New(nil), logger.New(100), wh,
		&stubHTTP{},
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
	return base
}

func TestAPI_WebhooksDisabledReturnsEmptyHistory(t *testing.T) {
	base := startAPIWithWebhooks(t, nil)

	resp, err := http.Get(base + "/api/webhooks")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got []webhook.Record
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty history when webhooks disabled, got %d", len(got))
	}
}

func TestAPI_SendWebhookAdHoc(t *testing.T) {
	received := make(chan struct{}, 1)
	callback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer callback.Close()

	wh := webhook.New(10)
	base := startAPIWithWebhooks(t, wh)

	reqBody, _ := json.Marshal(map[string]string{"url": callback.URL, "method": "POST"})
	resp, err := http.Post(base+"/api/webhooks/send", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("expected callback was not received")
	}

	histResp, err := http.Get(base + "/api/webhooks")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer histResp.Body.Close() //nolint:errcheck
	var records []webhook.Record
	if err := json.NewDecoder(histResp.Body).Decode(&records); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("history len = %d, want 1", len(records))
	}

	// DELETE clears the history.
	delReq, _ := http.NewRequest(http.MethodDelete, base+"/api/webhooks", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE error: %v", err)
	}
	defer delResp.Body.Close() //nolint:errcheck
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", delResp.StatusCode)
	}

	if len(wh.History().All()) != 0 {
		t.Error("expected webhook history to be cleared")
	}
}

func TestAPI_SendWebhookRequiresURL(t *testing.T) {
	base := startAPIWithWebhooks(t, webhook.New(10))

	resp, err := http.Post(base+"/api/webhooks/send", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("POST error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
