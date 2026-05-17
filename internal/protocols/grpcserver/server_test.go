package grpcserver_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/protocols/grpcserver"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// New / SetMocks / GetMocks
// ---------------------------------------------------------------------------

func TestNew_InitialMocks(t *testing.T) {
	cfg := &config.GRPCConfig{
		Enabled: true,
		Port:    0,
		Services: []config.GRPCService{
			{Mocks: []config.GRPCMock{{ID: "m1", Method: "GetUser"}}},
			{Mocks: []config.GRPCMock{{ID: "m2", Method: "ListUsers"}}},
		},
	}
	srv := grpcserver.New(cfg, state.New(), logger.New(100))
	mocks := srv.GetMocks()
	if len(mocks) != 2 {
		t.Fatalf("expected 2 mocks from New, got %d", len(mocks))
	}
}

func TestSetMocks_ReplacesList(t *testing.T) {
	srv := newTestServer(t, []config.GRPCMock{{ID: "old", Method: "OldMethod"}})

	newMocks := []config.GRPCMock{{ID: "new1", Method: "A"}, {ID: "new2", Method: "B"}}
	srv.SetMocks(newMocks)

	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("expected 2 mocks after SetMocks, got %d", len(got))
	}
	if got[0].ID != "new1" || got[1].ID != "new2" {
		t.Errorf("unexpected mocks: %+v", got)
	}
}

func TestSetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestServer(t, nil)
	original := []config.GRPCMock{{ID: "m1", Method: "Foo"}}
	srv.SetMocks(original)

	// Mutating the original slice must not affect the server's copy.
	original[0].Method = "Mutated"
	if srv.GetMocks()[0].Method != "Foo" {
		t.Error("SetMocks should copy the slice, not keep the reference")
	}
}

func TestGetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestServer(t, []config.GRPCMock{{ID: "m1", Method: "Bar"}})
	got := srv.GetMocks()
	got[0].Method = "Mutated"
	if srv.GetMocks()[0].Method != "Bar" {
		t.Error("GetMocks should return a copy, not the internal slice")
	}
}

// ---------------------------------------------------------------------------
// StatusInfo
// ---------------------------------------------------------------------------

func TestStatusInfo(t *testing.T) {
	cfg := &config.GRPCConfig{Enabled: true, Port: 50051}
	srv := grpcserver.New(cfg, state.New(), logger.New(100))
	srv.SetMocks([]config.GRPCMock{{ID: "x"}, {ID: "y"}})

	info := srv.StatusInfo()
	if info["protocol"] != "grpc" {
		t.Errorf("unexpected protocol %v", info["protocol"])
	}
	if info["enabled"] != true {
		t.Errorf("unexpected enabled %v", info["enabled"])
	}
	if info["port"] != 50051 {
		t.Errorf("unexpected port %v", info["port"])
	}
	if info["mocks"] != 2 {
		t.Errorf("unexpected mocks count %v", info["mocks"])
	}
}

// ---------------------------------------------------------------------------
// Integration — real gRPC round-trips
// ---------------------------------------------------------------------------

func TestGRPCServer_MockedMethod(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:       "get-user",
		Method:   "GetUser",
		Response: map[string]interface{}{"id": "123", "name": "Alice"},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	err = conn.Invoke(context.Background(), "/example.UserService/GetUser", []byte(`{}`), &resp)
	if err != nil {
		t.Fatalf("Invoke GetUser: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["id"] != "123" || body["name"] != "Alice" {
		t.Errorf("unexpected response body: %v", body)
	}
}

func TestGRPCServer_UnmockedMethod(t *testing.T) {
	addr := startGRPCServer(t, nil)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	err = conn.Invoke(context.Background(), "/svc/UnknownMethod", []byte(`{}`), &resp)
	if err == nil {
		t.Fatal("expected Unimplemented error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unimplemented {
		t.Errorf("expected Unimplemented, got: %v", err)
	}
}

func TestGRPCServer_WildcardMethod(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:       "wildcard",
		Method:   "*",
		Response: map[string]interface{}{"ok": true},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	if err := conn.Invoke(context.Background(), "/any.Service/AnyMethod", []byte(`{}`), &resp); err != nil {
		t.Fatalf("Invoke with wildcard mock: %v", err)
	}
}

func TestGRPCServer_ErrorMock(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "err-mock",
		Method: "FailMethod",
		Error:  &config.GRPCError{Code: int(codes.NotFound), Message: "not found"},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	err = conn.Invoke(context.Background(), "/svc/FailMethod", []byte(`{}`), &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
	if st.Message() != "not found" {
		t.Errorf("unexpected message %q", st.Message())
	}
}

// TestGRPCServer_ErrorMock_OutOfRangeCode validates that safeGRPCCode clamps
// out-of-range values to Unknown (2) rather than panicking.
func TestGRPCServer_ErrorMock_OutOfRangeCode(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "bad-code",
		Method: "BadCode",
		Error:  &config.GRPCError{Code: 999, Message: "bad code"},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	err = conn.Invoke(context.Background(), "/svc/BadCode", []byte(`{}`), &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unknown {
		t.Errorf("expected Unknown for out-of-range code 999, got %v", st.Code())
	}
}

// TestGRPCServer_ErrorMock_NegativeCode validates negative codes map to Unknown.
func TestGRPCServer_ErrorMock_NegativeCode(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "neg-code",
		Method: "NegCode",
		Error:  &config.GRPCError{Code: -1, Message: "negative"},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	var resp []byte
	err = conn.Invoke(context.Background(), "/svc/NegCode", []byte(`{}`), &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unknown {
		t.Errorf("expected Unknown for code -1, got %v", st.Code())
	}
}

func TestGRPCServer_DelayMock(t *testing.T) {
	d := config.Duration{}
	d.Duration = 50 * time.Millisecond
	mocks := []config.GRPCMock{{
		ID:       "slow",
		Method:   "SlowMethod",
		Delay:    d,
		Response: map[string]interface{}{"slow": true},
	}}
	addr := startGRPCServer(t, mocks)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	start := time.Now()
	var resp []byte
	if err := conn.Invoke(context.Background(), "/svc/SlowMethod", []byte(`{}`), &resp); err != nil {
		t.Fatalf("Invoke SlowMethod: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("expected delay >=40ms, got %v", elapsed)
	}
}

func TestGRPCServer_SetMocks_LiveUpdate(t *testing.T) {
	srv, addr := startGRPCServerRaw(t, nil)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	// Initially unmocked → Unimplemented.
	var resp []byte
	err = conn.Invoke(context.Background(), "/svc/DynMethod", []byte(`{}`), &resp)
	if st, _ := status.FromError(err); st.Code() != codes.Unimplemented {
		t.Fatalf("expected Unimplemented before SetMocks, got %v", err)
	}

	// Add a mock live.
	srv.SetMocks([]config.GRPCMock{{
		ID:       "dyn",
		Method:   "DynMethod",
		Response: map[string]interface{}{"dynamic": true},
	}})

	if err := conn.Invoke(context.Background(), "/svc/DynMethod", []byte(`{}`), &resp); err != nil {
		t.Fatalf("Invoke after SetMocks: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, mocks []config.GRPCMock) *grpcserver.Server {
	t.Helper()
	cfg := &config.GRPCConfig{Enabled: true, Port: 0}
	srv := grpcserver.New(cfg, state.New(), logger.New(100))
	if mocks != nil {
		srv.SetMocks(mocks)
	}
	return srv
}

// startGRPCServer starts a server with the given mocks on a free port and returns its address.
func startGRPCServer(t *testing.T, mocks []config.GRPCMock) string {
	t.Helper()
	_, addr := startGRPCServerRaw(t, mocks)
	return addr
}

func startGRPCServerRaw(t *testing.T, mocks []config.GRPCMock) (*grpcserver.Server, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.GRPCConfig{Enabled: true, Port: port}
	srv := grpcserver.New(cfg, state.New(), logger.New(100))
	if mocks != nil {
		srv.SetMocks(mocks)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck

	addr := net.JoinHostPort("127.0.0.1", itoa(port))
	waitForGRPC(t, addr, 2*time.Second)
	return srv, addr
}

func waitForGRPC(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("gRPC server at %s did not become ready within %v", addr, timeout)
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{digits[n%10]}, buf...)
		n /= 10
	}
	return string(buf)
}
