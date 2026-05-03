// Internal package test so we can access unexported helpers.
package graphqlserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/dever-labs/mockly/internal/testutil"
)

func TestExtractOperationType(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"query GetUser { user { id } }", "query"},
		{"mutation CreateUser { createUser { id } }", "mutation"},
		{"subscription OnUserCreated { userCreated { id } }", "subscription"},
		{"{ user { id } }", "query"}, // shorthand query
		{"  mutation  UpdateUser {}", "mutation"},
		{"", "query"},
	}
	for _, c := range cases {
		got := extractOperationType(c.query)
		if got != c.want {
			t.Errorf("extractOperationType(%q) = %q, want %q", c.query, got, c.want)
		}
	}
}

func TestMatchPattern_Exact(t *testing.T) {
	if !matchPattern("GetUser", "GetUser") {
		t.Error("exact match should succeed")
	}
	if matchPattern("GetUser", "GetUsers") {
		t.Error("exact match should fail for different names")
	}
}

func TestMatchPattern_Wildcard(t *testing.T) {
	if !matchPattern("Get*", "GetUser") {
		t.Error("wildcard prefix should match")
	}
	if !matchPattern("Get*", "GetAllUsers") {
		t.Error("wildcard should match any suffix")
	}
	if matchPattern("Get*", "CreateUser") {
		t.Error("wildcard prefix should not match different prefix")
	}
}

func TestMatchPattern_Regex(t *testing.T) {
	if !matchPattern(`re:^Get\w+$`, "GetUser") {
		t.Error("regex should match")
	}
	if matchPattern(`re:^Get\w+$`, "CreateUser") {
		t.Error("regex should not match")
	}
	if matchPattern("re:[bad", "anything") {
		t.Error("invalid regex should not match")
	}
}

func TestMatchMock_ByOperationName(t *testing.T) {
	sc := scenarios.New(nil)
	st := state.New()
	mocks := []config.GraphQLMock{
		{ID: "m1", OperationName: "GetUser", Response: map[string]interface{}{"user": "alice"}},
		{ID: "m2", OperationName: "CreateUser"},
	}
	srv := &Server{mocks: mocks, store: st, scenarios: sc}

	m, ok := srv.matchMock("query", "GetUser")
	if !ok || m.ID != "m1" {
		t.Fatalf("expected m1, got %v ok=%v", m.ID, ok)
	}

	_, notOk := srv.matchMock("query", "DeleteUser")
	if notOk {
		t.Fatal("expected no match for unknown operation")
	}
}

func TestMatchMock_ByOperationType(t *testing.T) {
	sc := scenarios.New(nil)
	st := state.New()
	mocks := []config.GraphQLMock{
		{ID: "m1", OperationType: "mutation", OperationName: "CreateUser"},
	}
	srv := &Server{mocks: mocks, store: st, scenarios: sc}

	_, ok := srv.matchMock("query", "CreateUser")
	if ok {
		t.Fatal("should not match query when operation_type is mutation")
	}

	m, ok2 := srv.matchMock("mutation", "CreateUser")
	if !ok2 || m.ID != "m1" {
		t.Fatalf("should match mutation CreateUser, got ok=%v", ok2)
	}
}

func TestMatchMock_AnyOperation(t *testing.T) {
	sc := scenarios.New(nil)
	st := state.New()
	mocks := []config.GraphQLMock{
		{ID: "catchall"},
	}
	srv := &Server{mocks: mocks, store: st, scenarios: sc}

	m, ok := srv.matchMock("mutation", "AnyName")
	if !ok || m.ID != "catchall" {
		t.Fatal("empty operation_type and operation_name should match any")
	}
}

func TestMatchMock_StateCondition(t *testing.T) {
	sc := scenarios.New(nil)
	st := state.New()
	mocks := []config.GraphQLMock{
		{
			ID:            "m1",
			OperationName: "GetUser",
			State:         &config.StateCondition{Key: "auth", Value: "true"},
		},
	}
	srv := &Server{mocks: mocks, store: st, scenarios: sc}

	_, ok := srv.matchMock("query", "GetUser")
	if ok {
		t.Fatal("should not match when state condition not met")
	}

	st.Set("auth", "true")
	m, ok2 := srv.matchMock("query", "GetUser")
	if !ok2 || m.ID != "m1" {
		t.Fatal("should match after state condition is set")
	}
}

func TestMatchMock_WildcardOperationName(t *testing.T) {
	sc := scenarios.New(nil)
	st := state.New()
	mocks := []config.GraphQLMock{
		{ID: "list-ops", OperationName: "List*"},
	}
	srv := &Server{mocks: mocks, store: st, scenarios: sc}

	m, ok := srv.matchMock("query", "ListUsers")
	if !ok || m.ID != "list-ops" {
		t.Fatal("wildcard should match ListUsers")
	}

	_, ok2 := srv.matchMock("query", "GetUser")
	if ok2 {
		t.Fatal("wildcard List* should not match GetUser")
	}
}

// ---------------------------------------------------------------------------
// Concurrency / race-detector tests
// ---------------------------------------------------------------------------

func TestMatchMock_ConcurrentAccess(t *testing.T) {
sc := scenarios.New(nil)
st := state.New()
srv := &Server{
mocks:     []config.GraphQLMock{{ID: "m1", OperationName: "GetUser"}},
store:     st,
scenarios: sc,
log:       logger.New(10),
}
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
wg.Add(2)
go func() {
defer wg.Done()
for j := 0; j < 100; j++ {
srv.SetMocks([]config.GraphQLMock{{ID: "updated", OperationName: "GetUser"}})
}
}()
go func() {
defer wg.Done()
for j := 0; j < 100; j++ {
srv.matchMock("query", "GetUser")
}
}()
}
wg.Wait()
}

// ---------------------------------------------------------------------------
// TLS tests
// ---------------------------------------------------------------------------

func TestGraphQLServer_TLS(t *testing.T) {
dir := t.TempDir()
certFile := dir + "/cert.pem"
keyFile := dir + "/key.pem"
if err := testutil.WriteSelfSignedCert(certFile, keyFile); err != nil {
t.Fatalf("generate cert: %v", err)
}

ln, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {
t.Fatalf("listen: %v", err)
}
port := ln.Addr().(*net.TCPAddr).Port
_ = ln.Close()

cfg := &config.GraphQLConfig{
Enabled: true,
Port:    port,
Path:    "/graphql",
TLS:     &config.TLSConfig{Enabled: true, CertFile: certFile, KeyFile: keyFile},
Mocks: []config.GraphQLMock{{
ID:            "ping",
OperationName: "Ping",
Response:      map[string]interface{}{"pong": true},
}},
}
srv := New(cfg, state.New(), scenarios.New(nil), logger.New(10))

ctx, cancel := context.WithCancel(context.Background())
t.Cleanup(cancel)
go srv.Start(ctx) //nolint:errcheck

client := &http.Client{
Transport: &http.Transport{
TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
},
}
base := fmt.Sprintf("https://127.0.0.1:%d", port)

deadline := time.Now().Add(2 * time.Second)
for time.Now().Before(deadline) {
resp, err := client.Get(base + "/graphql")
if err == nil {
_ = resp.Body.Close()
break
}
time.Sleep(10 * time.Millisecond)
}

body := `{"query":"query Ping { ping }","operationName":"Ping"}`
resp, err := client.Post(base+"/graphql", "application/json", strings.NewReader(body))
if err != nil {
t.Fatalf("POST /graphql over TLS: %v", err)
}
defer resp.Body.Close() //nolint:errcheck

if resp.StatusCode != 200 {
t.Errorf("want 200, got %d", resp.StatusCode)
}
respBody, err := io.ReadAll(resp.Body)
if err != nil {
t.Fatalf("read body: %v", err)
}
if !strings.Contains(string(respBody), "pong") {
t.Errorf("expected 'pong' in response, got %q", respBody)
}
}
