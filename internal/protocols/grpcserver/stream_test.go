package grpcserver_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/dever-labs/mockly/internal/config"
)

// serverStreamDesc/clientStreamDesc/bidiStreamDesc describe the three
// streaming cardinalities for grpc.ClientConn.NewStream; mockly's
// UnknownServiceHandler doesn't care which was declared (it reads the
// mock's ClientStream/Stream fields instead), but the real grpc-go client
// transport does care about framing, so tests must pick the matching one.
var (
	serverStreamDesc = &grpc.StreamDesc{StreamName: "ServerStream", ServerStreams: true}
	clientStreamDesc = &grpc.StreamDesc{StreamName: "ClientStream", ClientStreams: true}
	bidiStreamDesc   = &grpc.StreamDesc{StreamName: "BidiStream", ClientStreams: true, ServerStreams: true}
)

func dialTestServer(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func recvJSON(t *testing.T, stream grpc.ClientStream) map[string]interface{} {
	t.Helper()
	var resp []byte
	if err := stream.RecvMsg(&resp); err != nil {
		t.Fatalf("RecvMsg: %v", err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(resp, &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return body
}

func TestGRPCServer_ServerStream(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "list-items",
		Method: "ListItems",
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
			{Response: map[string]interface{}{"n": 2}},
			{Response: map[string]interface{}{"n": 3}},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/ListItems")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg request: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	for i := 1; i <= 3; i++ {
		body := recvJSON(t, stream)
		if int(body["n"].(float64)) != i {
			t.Errorf("message %d: n = %v, want %d", i, body["n"], i)
		}
	}

	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF after last message, got %v", err)
	}
}

func TestGRPCServer_ServerStream_ErrorMidStream(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "list-items-fail",
		Method: "ListItemsFail",
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
			{Error: &config.GRPCError{Code: int(codes.Internal), Message: "boom"}},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/ListItemsFail")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg request: %v", err)
	}
	_ = stream.CloseSend()

	body := recvJSON(t, stream)
	if int(body["n"].(float64)) != 1 {
		t.Errorf("message 1: n = %v, want 1", body["n"])
	}

	var resp []byte
	err = stream.RecvMsg(&resp)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal || st.Message() != "boom" {
		t.Errorf("expected Internal 'boom', got %v", err)
	}
}

func TestGRPCServer_ClientStream(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:           "sum-items",
		Method:       "SumItems",
		ClientStream: true,
		Response:     map[string]interface{}{"total": 6},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), clientStreamDesc, "/svc/SumItems")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if err := stream.SendMsg([]byte(`{}`)); err != nil {
			t.Fatalf("SendMsg %d: %v", i, err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	body := recvJSON(t, stream)
	if int(body["total"].(float64)) != 6 {
		t.Errorf("total = %v, want 6", body["total"])
	}
}

func TestGRPCServer_BidiStream(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:           "echo-items",
		Method:       "EchoItems",
		ClientStream: true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
			{Response: map[string]interface{}{"n": 2}},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), bidiStreamDesc, "/svc/EchoItems")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	for i := 1; i <= 2; i++ {
		if err := stream.SendMsg([]byte(`{}`)); err != nil {
			t.Fatalf("SendMsg %d: %v", i, err)
		}
		body := recvJSON(t, stream)
		if int(body["n"].(float64)) != i {
			t.Errorf("reply %d: n = %v, want %d", i, body["n"], i)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF after stream exhausted, got %v", err)
	}
}

func TestGRPCServer_BidiStream_ExtraClientMessagesAreDrained(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:           "echo-items-short",
		Method:       "EchoItemsShort",
		ClientStream: true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), bidiStreamDesc, "/svc/EchoItemsShort")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg 1: %v", err)
	}
	body := recvJSON(t, stream)
	if int(body["n"].(float64)) != 1 {
		t.Errorf("reply 1: n = %v, want 1", body["n"])
	}

	// Second client message has no corresponding Stream entry; the server
	// should keep draining silently rather than erroring, until CloseSend.
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg 2: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}

	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestGRPCServer_BidiStream_ErrorStopsStream(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:           "echo-items-fail",
		Method:       "EchoItemsFail",
		ClientStream: true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
			{Error: &config.GRPCError{Code: int(codes.Aborted), Message: "nope"}},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), bidiStreamDesc, "/svc/EchoItemsFail")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}

	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg 1: %v", err)
	}
	_ = recvJSON(t, stream)

	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg 2: %v", err)
	}
	var resp []byte
	err = stream.RecvMsg(&resp)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Aborted || st.Message() != "nope" {
		t.Errorf("expected Aborted 'nope', got %v", err)
	}
}

func TestGRPCServer_StreamDelay(t *testing.T) {
	d := config.Duration{}
	d.Duration = 50 * time.Millisecond
	mocks := []config.GRPCMock{{
		ID:     "slow-stream",
		Method: "SlowStream",
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}, Delay: d},
		},
	}}
	addr := startGRPCServer(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/SlowStream")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg: %v", err)
	}
	_ = stream.CloseSend()

	start := time.Now()
	_ = recvJSON(t, stream)
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("expected delay >=40ms, got %v", elapsed)
	}
}
