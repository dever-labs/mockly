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
	"github.com/dever-labs/mockly/internal/protocols/grpcserver"
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

func waitForStreamCount(t *testing.T, srv *grpcserver.Server, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(srv.ListStreams()) == n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d active streams, got %d", n, len(srv.ListStreams()))
}

func TestGRPCServer_ServerStream_KeepOpen_PushAndClose(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:       "keep-open-list",
		Method:   "KeepOpenList",
		KeepOpen: true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
		},
	}}
	srv, addr := startGRPCServerRaw(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/KeepOpenList")
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

	waitForStreamCount(t, srv, 1)
	info := srv.ListStreams()
	if info[0].Protocol != "grpc" || info[0].MockID != "keep-open-list" {
		t.Errorf("want protocol=grpc mockID=keep-open-list, got %+v", info[0])
	}

	n, err := srv.PushStream("", "keep-open-list", `{"n":2}`)
	if err != nil || n != 1 {
		t.Fatalf("PushStream: n=%d err=%v", n, err)
	}
	body = recvJSON(t, stream)
	if int(body["n"].(float64)) != 2 {
		t.Errorf("pushed message: n = %v, want 2", body["n"])
	}

	if _, err := srv.CloseStream("", "keep-open-list", ""); err != nil {
		t.Fatalf("CloseStream: %v", err)
	}
	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF after a clean close, got %v", err)
	}
}

func TestGRPCServer_ServerStream_KeepOpen_CloseWithError(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:       "keep-open-list-err",
		Method:   "KeepOpenListErr",
		KeepOpen: true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
		},
	}}
	srv, addr := startGRPCServerRaw(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/KeepOpenListErr")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg request: %v", err)
	}
	_ = stream.CloseSend()
	_ = recvJSON(t, stream)

	waitForStreamCount(t, srv, 1)
	if _, err := srv.CloseStream("", "keep-open-list-err", "boom"); err != nil {
		t.Fatalf("CloseStream: %v", err)
	}

	var resp []byte
	err = stream.RecvMsg(&resp)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Aborted || st.Message() != "boom" {
		t.Errorf("expected Aborted 'boom', got %v", err)
	}
}

func TestGRPCServer_BidiStream_KeepOpen_SurvivesHalfCloseAndPushes(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:           "keep-open-echo",
		Method:       "KeepOpenEcho",
		ClientStream: true,
		KeepOpen:     true,
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
		},
	}}
	srv, addr := startGRPCServerRaw(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), bidiStreamDesc, "/svc/KeepOpenEcho")
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

	// Half-closing the client side must NOT end a KeepOpen bidi call.
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
	waitForStreamCount(t, srv, 1)

	n, err := srv.PushStream("", "keep-open-echo", `{"n":2}`)
	if err != nil || n != 1 {
		t.Fatalf("PushStream: n=%d err=%v", n, err)
	}
	body = recvJSON(t, stream)
	if int(body["n"].(float64)) != 2 {
		t.Errorf("pushed message: n = %v, want 2", body["n"])
	}

	if _, err := srv.CloseStream("", "keep-open-echo", ""); err != nil {
		t.Fatalf("CloseStream: %v", err)
	}
	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF after a clean close, got %v", err)
	}
}

func TestGRPCServer_Stream_DefaultKeepOpenFalseEndsCall(t *testing.T) {
	mocks := []config.GRPCMock{{
		ID:     "no-keep-open",
		Method: "NoKeepOpen",
		Stream: []config.GRPCStreamMessage{
			{Response: map[string]interface{}{"n": 1}},
		},
	}}
	srv, addr := startGRPCServerRaw(t, mocks)
	conn := dialTestServer(t, addr)

	stream, err := conn.NewStream(context.Background(), serverStreamDesc, "/svc/NoKeepOpen")
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg: %v", err)
	}
	_ = stream.CloseSend()
	_ = recvJSON(t, stream)

	var trailing []byte
	if err := stream.RecvMsg(&trailing); err != io.EOF {
		t.Errorf("expected io.EOF (call ends without keep_open), got %v", err)
	}
	if got := srv.ListStreams(); len(got) != 0 {
		t.Errorf("want no registered streams for a non-keep_open stream, got %d", len(got))
	}
}
