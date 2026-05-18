package imapserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
)

func TestFetchMessages(t *testing.T) {
	msgs := []config.IMAPMessage{{SeqNum: 1}, {SeqNum: 2}}
	if got := fetchMessages(msgs, "1:*"); len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
	if got := fetchMessages(msgs, "2"); len(got) != 1 || got[0].SeqNum != 2 {
		t.Fatalf("unexpected fetch result: %#v", got)
	}
}

func TestStatusInfo(t *testing.T) {
	srv := New(&config.IMAPConfig{Enabled: true, Port: 1143, Mailboxes: []config.IMAPMailbox{{ID: "1"}}}, nil, nil)
	info := srv.StatusInfo()
	if info["protocol"] != "imap" || info["port"] != 1143 {
		t.Fatalf("unexpected status info: %#v", info)
	}
}
