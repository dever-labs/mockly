// White-box unit tests for imapserver helpers.
package imapserver

import (
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
)

func newTestIMAPServer(mailboxes []config.IMAPMailbox) *Server {
	cfg := &config.IMAPConfig{Enabled: true, Port: 1143, Mailboxes: mailboxes}
	return New(cfg, scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// SetMailboxes / GetMailboxes
// ---------------------------------------------------------------------------

func TestIMAP_SetGetMailboxes(t *testing.T) {
	srv := newTestIMAPServer(nil)
	mailboxes := []config.IMAPMailbox{
		{ID: "mb1", Name: "INBOX"},
		{ID: "mb2", Name: "Sent"},
	}
	srv.SetMailboxes(mailboxes)
	got := srv.GetMailboxes()
	if len(got) != 2 {
		t.Fatalf("want 2 mailboxes, got %d", len(got))
	}
	if got[0].ID != "mb1" || got[1].ID != "mb2" {
		t.Errorf("unexpected mailboxes: %+v", got)
	}
}

func TestIMAP_GetMailboxes_IsolatesSlice(t *testing.T) {
	srv := newTestIMAPServer([]config.IMAPMailbox{{ID: "mb1", Name: "INBOX"}})
	got := srv.GetMailboxes()
	got[0].ID = "mutated"
	if srv.GetMailboxes()[0].ID != "mb1" {
		t.Error("GetMailboxes should return a copy")
	}
}

// ---------------------------------------------------------------------------
// quoteMailbox
// ---------------------------------------------------------------------------

func TestQuoteMailbox_AlreadyQuoted(t *testing.T) {
	got := quoteMailbox(`"INBOX"`)
	if got != `"INBOX"` {
		t.Errorf("already-quoted mailbox should be unchanged, got %q", got)
	}
}

func TestQuoteMailbox_NotQuoted(t *testing.T) {
	got := quoteMailbox("INBOX")
	if got != `"INBOX"` {
		t.Errorf("unquoted mailbox should be wrapped in quotes, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// authenticate
// ---------------------------------------------------------------------------

func TestIMAP_Authenticate_NoUsers(t *testing.T) {
	cfg := &config.IMAPConfig{Enabled: true, Port: 1143}
	srv := New(cfg, scenarios.New(nil), logger.New(100))
	// No users configured → always authenticate.
	if !srv.authenticate([]string{"anyone", "anypass"}) {
		t.Error("should authenticate when no users configured")
	}
}

func TestIMAP_Authenticate_ValidCredentials(t *testing.T) {
	cfg := &config.IMAPConfig{
		Enabled: true, Port: 1143,
		Users: []config.IMAPUser{{Username: "alice", Password: "secret"}},
	}
	srv := New(cfg, scenarios.New(nil), logger.New(100))
	if !srv.authenticate([]string{"alice", "secret"}) {
		t.Error("valid credentials should authenticate")
	}
}

func TestIMAP_Authenticate_WrongPassword(t *testing.T) {
	cfg := &config.IMAPConfig{
		Enabled: true, Port: 1143,
		Users: []config.IMAPUser{{Username: "alice", Password: "secret"}},
	}
	srv := New(cfg, scenarios.New(nil), logger.New(100))
	if srv.authenticate([]string{"alice", "wrong"}) {
		t.Error("wrong password should fail authentication")
	}
}

func TestIMAP_Authenticate_WrongUser(t *testing.T) {
	cfg := &config.IMAPConfig{
		Enabled: true, Port: 1143,
		Users: []config.IMAPUser{{Username: "alice", Password: "secret"}},
	}
	srv := New(cfg, scenarios.New(nil), logger.New(100))
	if srv.authenticate([]string{"bob", "secret"}) {
		t.Error("unknown user should fail authentication")
	}
}

func TestIMAP_Authenticate_TooFewArgs(t *testing.T) {
	cfg := &config.IMAPConfig{
		Enabled: true, Port: 1143,
		Users: []config.IMAPUser{{Username: "alice", Password: "secret"}},
	}
	srv := New(cfg, scenarios.New(nil), logger.New(100))
	if srv.authenticate([]string{"alice"}) {
		t.Error("too few args should fail authentication")
	}
}

// ---------------------------------------------------------------------------
// buildFetchResponse
// ---------------------------------------------------------------------------

func TestBuildFetchResponse_Flags(t *testing.T) {
	msg := config.IMAPMessage{SeqNum: 1, Flags: []string{`\Seen`}}
	resp := buildFetchResponse(msg, "FLAGS")
	if !strings.Contains(resp, "FLAGS") {
		t.Errorf("response should contain FLAGS, got: %q", resp)
	}
	if !strings.Contains(resp, `\Seen`) {
		t.Errorf("response should contain \\Seen flag, got: %q", resp)
	}
}

func TestBuildFetchResponse_RFC822Size(t *testing.T) {
	msg := config.IMAPMessage{SeqNum: 1, Body: "hello"}
	resp := buildFetchResponse(msg, "RFC822.SIZE")
	if !strings.Contains(resp, "RFC822.SIZE") {
		t.Errorf("response should contain RFC822.SIZE, got: %q", resp)
	}
}

func TestBuildFetchResponse_BodyHeaderFields(t *testing.T) {
	msg := config.IMAPMessage{SeqNum: 1, From: "a@a.com", To: "b@b.com", Subject: "Hi", Date: "Mon"}
	resp := buildFetchResponse(msg, "BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)]")
	if !strings.Contains(resp, "BODY[HEADER.FIELDS") {
		t.Errorf("response should contain BODY[HEADER.FIELDS, got: %q", resp)
	}
	if !strings.Contains(resp, "From:") {
		t.Errorf("response should contain From header, got: %q", resp)
	}
}

func TestBuildFetchResponse_BodyFull(t *testing.T) {
	msg := config.IMAPMessage{SeqNum: 1, Body: "email body text"}
	resp := buildFetchResponse(msg, "BODY[]")
	if !strings.Contains(resp, "BODY[]") {
		t.Errorf("response should contain BODY[], got: %q", resp)
	}
	if !strings.Contains(resp, "email body text") {
		t.Errorf("response should contain body content, got: %q", resp)
	}
}

func TestBuildFetchResponse_BodyText(t *testing.T) {
	msg := config.IMAPMessage{SeqNum: 1, Body: "just the body"}
	resp := buildFetchResponse(msg, "BODY[TEXT]")
	if !strings.Contains(resp, "BODY[TEXT]") {
		t.Errorf("response should contain BODY[TEXT], got: %q", resp)
	}
	if !strings.Contains(resp, "just the body") {
		t.Errorf("response should contain body text, got: %q", resp)
	}
}

// ---------------------------------------------------------------------------
// fetchMessages extras
// ---------------------------------------------------------------------------

func TestFetchMessages_Star(t *testing.T) {
	msgs := []config.IMAPMessage{{SeqNum: 1}, {SeqNum: 2}, {SeqNum: 3}}
	got := fetchMessages(msgs, "*")
	if len(got) != 3 {
		t.Errorf("'*' should return all messages, got %d", len(got))
	}
}

func TestFetchMessages_Empty(t *testing.T) {
	msgs := []config.IMAPMessage{{SeqNum: 1}, {SeqNum: 2}}
	got := fetchMessages(msgs, "")
	if len(got) != 2 {
		t.Errorf("empty seq should return all messages, got %d", len(got))
	}
}

func TestFetchMessages_NotFound(t *testing.T) {
	msgs := []config.IMAPMessage{{SeqNum: 1}}
	got := fetchMessages(msgs, "99")
	if len(got) != 0 {
		t.Errorf("non-existent SeqNum should return empty, got %d", len(got))
	}
}
