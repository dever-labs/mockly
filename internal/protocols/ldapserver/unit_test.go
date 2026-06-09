// White-box unit tests for ldapserver helpers.
package ldapserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

func newTestLDAPServer(mocks []config.LDAPMock) *Server {
	cfg := &config.LDAPConfig{Enabled: true, Port: 3893, Mocks: mocks}
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// SetMocks / GetMocks
// ---------------------------------------------------------------------------

func TestLDAP_SetGetMocks(t *testing.T) {
	srv := newTestLDAPServer(nil)
	mocks := []config.LDAPMock{
		{ID: "m1", BaseDN: "dc=example,dc=com"},
		{ID: "m2", BaseDN: "ou=users,dc=example,dc=com"},
	}
	srv.SetMocks(mocks)
	got := srv.GetMocks()
	if len(got) != 2 {
		t.Fatalf("want 2 mocks, got %d", len(got))
	}
}

func TestLDAP_GetMocks_IsolatesSlice(t *testing.T) {
	srv := newTestLDAPServer([]config.LDAPMock{{ID: "m1"}})
	got := srv.GetMocks()
	got[0].ID = "mutated"
	if srv.GetMocks()[0].ID != "m1" {
		t.Error("GetMocks should return a copy")
	}
}

// ---------------------------------------------------------------------------
// matchMocks
// ---------------------------------------------------------------------------

func TestLDAP_MatchMocks_NoMocks(t *testing.T) {
	srv := newTestLDAPServer(nil)
	got := srv.matchMocks("dc=example,dc=com", "")
	if len(got) != 0 {
		t.Errorf("should return empty when no mocks, got %d", len(got))
	}
}

func TestLDAP_MatchMocks_BaseDNMismatch(t *testing.T) {
	srv := newTestLDAPServer([]config.LDAPMock{
		{ID: "m1", BaseDN: "dc=other,dc=com"},
	})
	got := srv.matchMocks("dc=example,dc=com", "")
	if len(got) != 0 {
		t.Errorf("should not match different BaseDN, got %d", len(got))
	}
}

func TestLDAP_MatchMocks_FilterMismatch(t *testing.T) {
	srv := newTestLDAPServer([]config.LDAPMock{
		{ID: "m1", BaseDN: "dc=example,dc=com", Filter: "(uid=alice)"},
	})
	got := srv.matchMocks("dc=example,dc=com", "(uid=bob)")
	if len(got) != 0 {
		t.Errorf("should not match different filter, got %d", len(got))
	}
}

func TestLDAP_MatchMocks_Match(t *testing.T) {
	srv := newTestLDAPServer([]config.LDAPMock{
		{ID: "m1", BaseDN: "dc=example,dc=com", Filter: "(uid=alice)"},
	})
	got := srv.matchMocks("dc=example,dc=com", "(uid=alice)")
	if len(got) != 1 || got[0].ID != "m1" {
		t.Errorf("should match, got %+v", got)
	}
}

func TestLDAP_MatchMocks_EmptyFilter(t *testing.T) {
	srv := newTestLDAPServer([]config.LDAPMock{
		{ID: "m1", BaseDN: "dc=example,dc=com"},
	})
	// Empty filter in mock matches any filter.
	got := srv.matchMocks("dc=example,dc=com", "(uid=anyone)")
	if len(got) != 1 {
		t.Errorf("empty filter should match any filter, got %d", len(got))
	}
}

func TestLDAP_MatchMocks_StateCondition(t *testing.T) {
	st := state.New()
	cfg := &config.LDAPConfig{Enabled: true, Port: 3893, Mocks: []config.LDAPMock{
		{ID: "m1", BaseDN: "dc=example,dc=com", State: &config.StateCondition{Key: "ldap_mode", Value: "active"}},
	}}
	srv := New(cfg, st, scenarios.New(nil), logger.New(100))

	if got := srv.matchMocks("dc=example,dc=com", ""); len(got) != 0 {
		t.Error("should not match when state condition is unmet")
	}

	st.Set("ldap_mode", "active")
	if got := srv.matchMocks("dc=example,dc=com", ""); len(got) != 1 {
		t.Error("should match when state condition is met")
	}
}

// ---------------------------------------------------------------------------
// berLen extras
// ---------------------------------------------------------------------------

func TestBerLen_Short(t *testing.T) {
	got := berLen(0x7f)
	if len(got) != 1 || got[0] != 0x7f {
		t.Errorf("berLen(0x7f) should be single byte 0x7f, got %v", got)
	}
}

func TestBerLen_TwoByte(t *testing.T) {
	got := berLen(200)
	if len(got) != 2 || got[0] != 0x81 || got[1] != 200 {
		t.Errorf("berLen(200) = %v, want [0x81, 200]", got)
	}
}

func TestBerLen_ThreeByte(t *testing.T) {
	got := berLen(256)
	if len(got) != 3 || got[0] != 0x82 {
		t.Errorf("berLen(256) = %v, want 3-byte encoding", got)
	}
}

// ---------------------------------------------------------------------------
// tlvLength
// ---------------------------------------------------------------------------

func TestTlvLength_Empty(t *testing.T) {
	length, hdr := tlvLength(nil)
	if length != 0 || hdr != 0 {
		t.Errorf("tlvLength(nil) = (%d, %d), want (0, 0)", length, hdr)
	}
}

func TestTlvLength_Short(t *testing.T) {
	length, hdr := tlvLength([]byte{0x05})
	if length != 5 || hdr != 1 {
		t.Errorf("tlvLength short = (%d, %d), want (5, 1)", length, hdr)
	}
}

func TestTlvLength_LongForm1(t *testing.T) {
	// 0x81, 0xC8 → length=200, consumed=2 bytes
	length, hdr := tlvLength([]byte{0x81, 0xC8})
	if length != 200 || hdr != 2 {
		t.Errorf("tlvLength long-1 = (%d, %d), want (200, 2)", length, hdr)
	}
}

func TestTlvLength_LongForm2(t *testing.T) {
	// 0x82, 0x01, 0x00 → length=256, consumed=3 bytes
	length, hdr := tlvLength([]byte{0x82, 0x01, 0x00})
	if length != 256 || hdr != 3 {
		t.Errorf("tlvLength long-2 = (%d, %d), want (256, 3)", length, hdr)
	}
}

// ---------------------------------------------------------------------------
// readTLV
// ---------------------------------------------------------------------------

func TestReadTLV_TooShort(t *testing.T) {
	tag, value, consumed := readTLV([]byte{0x04})
	if tag != 0 || value != nil || consumed != 1 {
		t.Errorf("readTLV short input = (%d, %v, %d)", tag, value, consumed)
	}
}

func TestReadTLV_Normal(t *testing.T) {
	// tag=0x04, length=3, value="abc"
	input := []byte{0x04, 0x03, 'a', 'b', 'c'}
	tag, value, consumed := readTLV(input)
	if tag != 0x04 {
		t.Errorf("tag = 0x%02x, want 0x04", tag)
	}
	if string(value) != "abc" {
		t.Errorf("value = %q, want 'abc'", value)
	}
	if consumed != 5 {
		t.Errorf("consumed = %d, want 5", consumed)
	}
}

// ---------------------------------------------------------------------------
// encodeInt
// ---------------------------------------------------------------------------

func TestEncodeInt_Zero(t *testing.T) {
	got := encodeInt(0)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("encodeInt(0) = %v, want [0]", got)
	}
}

func TestEncodeInt_Small(t *testing.T) {
	got := encodeInt(42)
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("encodeInt(42) = %v, want [42]", got)
	}
}

func TestEncodeInt_TwoBytes(t *testing.T) {
	got := encodeInt(256)
	if len(got) != 2 || got[0] != 1 || got[1] != 0 {
		t.Errorf("encodeInt(256) = %v, want [1, 0]", got)
	}
}
