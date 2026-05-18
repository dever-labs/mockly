package grpcserver_test

import (
	"context"
	"fmt"
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
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func startServer(t *testing.T, srv interface{ Start(context.Context) error }) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.Start(ctx) //nolint:errcheck
	time.Sleep(100 * time.Millisecond)
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	time.Sleep(10 * time.Millisecond)
	return port
}

func invokeGRPC(t *testing.T, conn *grpc.ClientConn) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := []byte(`{"name":"test"}`)
	var resp []byte
	err := conn.Invoke(ctx, "/mock.Greeter/SayHello", &req, &resp)
	return resp, err
}

func TestGRPCServer_GlobalFault(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := grpcserver.New(&config.GRPCConfig{
		Enabled: true,
		Port:    port,
		Services: []config.GRPCService{{
			Proto: "service Greeter",
			Mocks: []config.GRPCMock{{
				ID:       "m",
				Method:   "SayHello",
				Response: map[string]interface{}{"message": "hi"},
			}},
		}},
	}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	sc.SetDirectFaults(config.ProtocolFaults{GRPC: &config.GRPCFault{ErrorRate: 0}})
	_, err = invokeGRPC(t, conn)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("fault status code = %v, want %v (err=%v)", status.Code(err), codes.Unavailable, err)
	}

	sc.ClearDirectFaults()
	resp, err := invokeGRPC(t, conn)
	if err != nil {
		t.Fatalf("invoke normal gRPC: %v", err)
	}
	if got := string(resp); got != `{"message":"hi"}` {
		t.Fatalf("normal response = %q, want %q", got, `{"message":"hi"}`)
	}
}

func TestGRPCServer_GRPCFault_NotFound(t *testing.T) {
	port := freePort(t)
	sc := scenarios.New(nil)
	srv := grpcserver.New(&config.GRPCConfig{Enabled: true, Port: port}, state.New(), sc, logger.New(100))
	startServer(t, srv)

	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", port), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	sc.SetDirectFaults(config.ProtocolFaults{GRPC: &config.GRPCFault{Code: "NOT_FOUND", ErrorRate: 0}})
	_, err = invokeGRPC(t, conn)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("fault status code = %v, want %v (err=%v)", status.Code(err), codes.NotFound, err)
	}
}
