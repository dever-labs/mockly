// Internal package test so we can access unexported helpers.
package smtpserver

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
)

func makeEmail(id, from string) config.ReceivedEmail {
	return config.ReceivedEmail{ID: id, From: from, To: []string{"to@example.com"}}
}

func TestInbox_AddAndAll(t *testing.T) {
	b := newInbox(100)
	b.Add(makeEmail("e1", "alice@example.com"))
	b.Add(makeEmail("e2", "bob@example.com"))

	all := b.All()
	if len(all) != 2 {
		t.Fatalf("want 2 emails, got %d", len(all))
	}
	if all[0].From != "alice@example.com" {
		t.Errorf("unexpected first email: %v", all[0])
	}
}

func TestInbox_CapacityEviction(t *testing.T) {
	b := newInbox(3)
	for i := 0; i < 5; i++ {
		b.Add(makeEmail(string(rune('a'+i)), "test@test.com"))
	}
	all := b.All()
	if len(all) != 3 {
		t.Fatalf("want 3 emails (capacity), got %d", len(all))
	}
	// First two (ID "a" and "b") should have been evicted.
	if all[0].ID != "c" {
		t.Errorf("oldest should be evicted; want first remaining ID 'c', got %q", all[0].ID)
	}
}

func TestInbox_Clear(t *testing.T) {
	b := newInbox(10)
	b.Add(makeEmail("e1", "a@b.com"))
	b.Clear()
	if len(b.All()) != 0 {
		t.Fatal("expected empty after Clear")
	}
}

func TestInbox_DefaultMaxSize(t *testing.T) {
	b := newInbox(0) // 0 should use default
	if b.maxSize <= 0 {
		t.Error("default maxSize should be positive")
	}
}

func TestMatchSMTPPattern_Exact(t *testing.T) {
	if !matchSMTPPattern("alice@example.com", "alice@example.com") {
		t.Error("exact match should succeed")
	}
	if !matchSMTPPattern("Alice@Example.com", "alice@example.com") {
		t.Error("pattern should be case-insensitive")
	}
	if matchSMTPPattern("bob@example.com", "alice@example.com") {
		t.Error("different address should not match")
	}
}

func TestMatchSMTPPattern_Wildcard(t *testing.T) {
	if !matchSMTPPattern("*@example.com", "alice@example.com") {
		t.Error("wildcard prefix should match")
	}
	if matchSMTPPattern("*@example.com", "alice@other.com") {
		t.Error("wildcard should not match different domain")
	}
}

func TestMatchSMTPPattern_Regex(t *testing.T) {
	if !matchSMTPPattern(`re:^\w+@example\.com$`, "user@example.com") {
		t.Error("regex should match valid address")
	}
	if matchSMTPPattern(`re:^\w+@example\.com$`, "user@other.com") {
		t.Error("regex should not match invalid address")
	}
	if matchSMTPPattern("re:[bad", "anything") {
		t.Error("invalid regex should not match")
	}
}

func TestMatchRule_NoRules_DefaultAccept(t *testing.T) {
	s := &Server{rules: nil}
	action, _ := s.matchRule("a@example.com", "b@example.com", "Hello")
	if action != "accept" {
		t.Errorf("no rules should default to accept, got %q", action)
	}
}

func TestMatchRule_Reject(t *testing.T) {
	s := &Server{rules: []config.SMTPRule{
		{ID: "r1", From: "spam@*", Action: "reject", Message: "spam not accepted"},
	}}
	action, msg := s.matchRule("spam@badactor.com", "me@example.com", "")
	if action != "reject" {
		t.Errorf("expected reject, got %q", action)
	}
	if msg != "spam not accepted" {
		t.Errorf("unexpected message %q", msg)
	}
}

func TestMatchRule_LegitSenderNotRejected(t *testing.T) {
	s := &Server{rules: []config.SMTPRule{
		{ID: "r1", From: "spam@*", Action: "reject"},
	}}
	action, _ := s.matchRule("legit@example.com", "me@example.com", "Hello")
	if action != "accept" {
		t.Errorf("legit sender should not be rejected, got %q", action)
	}
}

func TestMatchRule_RejectBySubject(t *testing.T) {
	s := &Server{rules: []config.SMTPRule{
		{ID: "r1", Subject: "re:(?i)spam", Action: "reject", Message: "spam subject"},
	}}
	action, _ := s.matchRule("anyone@example.com", "me@example.com", "SPAM OFFER")
	if action != "reject" {
		t.Errorf("subject regex reject should fire, got %q", action)
	}
}

// ---------------------------------------------------------------------------
// Concurrency / race-detector tests
// ---------------------------------------------------------------------------

func TestSMTPServer_SetRules_ConcurrentAccess(t *testing.T) {
srv := &Server{
rules: []config.SMTPRule{{ID: "r1", Action: "accept"}},
inbox: newInbox(10),
}
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
wg.Add(2)
go func() {
defer wg.Done()
for j := 0; j < 100; j++ {
srv.SetRules([]config.SMTPRule{{ID: "r", Action: "accept"}})
}
}()
go func() {
defer wg.Done()
for j := 0; j < 100; j++ {
srv.matchRule("a@b.com", "c@d.com", "hello")
}
}()
}
wg.Wait()
}

// ---------------------------------------------------------------------------
// NewInbox (exported constructor)
// ---------------------------------------------------------------------------

func TestNewInbox_Positive(t *testing.T) {
b := NewInbox(10)
if b == nil {
t.Fatal("NewInbox returned nil")
}
}

func TestNewInbox_Default(t *testing.T) {
b := NewInbox(0)
if b == nil {
t.Fatal("NewInbox(0) returned nil")
}
if b.maxSize <= 0 {
t.Error("default maxSize should be positive")
}
}

// ---------------------------------------------------------------------------
// New / GetRules / GetInbox / StatusInfo
// ---------------------------------------------------------------------------

func TestSMTP_New_InitialRules(t *testing.T) {
cfg := &config.SMTPConfig{
Enabled: true,
Port:    0,
Domain:  "test.local",
Rules: []config.SMTPRule{
{ID: "r1", Action: "accept"},
},
}
srv := New(cfg, logger.New(100))
rules := srv.GetRules()
if len(rules) != 1 || rules[0].ID != "r1" {
t.Fatalf("unexpected rules from New: %+v", rules)
}
}

func TestSMTP_GetInbox_NotNil(t *testing.T) {
cfg := &config.SMTPConfig{Domain: "test.local"}
srv := New(cfg, logger.New(10))
if srv.GetInbox() == nil {
t.Fatal("GetInbox should not return nil")
}
}

func TestSMTP_SetRules_IsolatesSlice(t *testing.T) {
cfg := &config.SMTPConfig{Domain: "test.local"}
srv := New(cfg, logger.New(10))
original := []config.SMTPRule{{ID: "r1", Action: "accept"}}
srv.SetRules(original)
original[0].Action = "reject"
if srv.GetRules()[0].Action != "accept" {
t.Error("SetRules should copy the slice")
}
}

func TestSMTP_StatusInfo(t *testing.T) {
cfg := &config.SMTPConfig{Enabled: true, Port: 2525, Domain: "mockly.local"}
srv := New(cfg, logger.New(100))
srv.SetRules([]config.SMTPRule{{ID: "r1"}, {ID: "r2"}})
srv.GetInbox().Add(config.ReceivedEmail{ID: "e1"})

info := srv.StatusInfo()
if info["protocol"] != "smtp" {
t.Errorf("unexpected protocol %v", info["protocol"])
}
if info["port"] != 2525 {
t.Errorf("want port 2525, got %v", info["port"])
}
if info["domain"] != "mockly.local" {
t.Errorf("want domain mockly.local, got %v", info["domain"])
}
if info["rules"] != 2 {
t.Errorf("want rules=2, got %v", info["rules"])
}
if info["emails"] != 1 {
t.Errorf("want emails=1, got %v", info["emails"])
}
}

// ---------------------------------------------------------------------------
// Integration — real SMTP send
// ---------------------------------------------------------------------------

func TestSMTP_Integration_ReceiveEmail(t *testing.T) {
port := freePort(t)
cfg := &config.SMTPConfig{
Enabled: true,
Port:    port,
Domain:  "test.local",
}
srv := New(cfg, logger.New(100))

ctx, cancel := context.WithCancel(context.Background())
t.Cleanup(cancel)
go srv.Start(ctx) //nolint:errcheck

waitForTCP(t, port, 2*time.Second)

addr := fmt.Sprintf("127.0.0.1:%d", port)
if err := smtp.SendMail(addr, nil, "alice@example.com", []string{"bob@example.com"}, []byte("Subject: Hello\r\n\r\nHello World")); err != nil {
t.Fatalf("SendMail: %v", err)
}

emails := srv.GetInbox().All()
if len(emails) != 1 {
t.Fatalf("expected 1 email in inbox, got %d", len(emails))
}
if emails[0].From != "alice@example.com" {
t.Errorf("unexpected from %q", emails[0].From)
}
if len(emails[0].To) != 1 || emails[0].To[0] != "bob@example.com" {
t.Errorf("unexpected to %v", emails[0].To)
}
}

func TestSMTP_Integration_RejectedEmail(t *testing.T) {
port := freePort(t)
cfg := &config.SMTPConfig{
Enabled: true,
Port:    port,
Domain:  "test.local",
Rules: []config.SMTPRule{
{ID: "block-spam", From: "spam@*", Action: "reject", Message: "spam not allowed"},
},
}
srv := New(cfg, logger.New(100))

ctx, cancel := context.WithCancel(context.Background())
t.Cleanup(cancel)
go srv.Start(ctx) //nolint:errcheck

waitForTCP(t, port, 2*time.Second)

addr := fmt.Sprintf("127.0.0.1:%d", port)
err := smtp.SendMail(addr, nil, "spam@badactor.com", []string{"me@example.com"}, []byte("Subject: Buy now\r\n\r\nspam"))
if err == nil {
t.Fatal("expected rejection error, got nil")
}

// Inbox should be empty — rejected email should not be stored.
if n := len(srv.GetInbox().All()); n != 0 {
t.Errorf("expected empty inbox after rejection, got %d", n)
}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func freePort(t *testing.T) int {
t.Helper()
ln, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil {
t.Fatalf("listen: %v", err)
}
port := ln.Addr().(*net.TCPAddr).Port
_ = ln.Close()
return port
}

func waitForTCP(t *testing.T, port int, timeout time.Duration) {
t.Helper()
addr := fmt.Sprintf("127.0.0.1:%d", port)
deadline := time.Now().Add(timeout)
for time.Now().Before(deadline) {
conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
if err == nil {
_ = conn.Close()
return
}
time.Sleep(20 * time.Millisecond)
}
t.Fatalf("server at %s not ready within %v", addr, timeout)
}
