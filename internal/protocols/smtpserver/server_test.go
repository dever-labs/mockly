// Internal package test so we can access unexported helpers.
package smtpserver

import (
	"sync"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
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
