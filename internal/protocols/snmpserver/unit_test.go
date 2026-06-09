// Additional white-box unit tests for snmpserver helpers.
// Supplements the existing server_test.go with missing cases.
package snmpserver

import (
	"testing"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
	"github.com/gosnmp/gosnmp"
)

func newTestSNMPServer(cfg *config.SNMPConfig) *Server {
	return New(cfg, state.New(), scenarios.New(nil), logger.New(100))
}

// ---------------------------------------------------------------------------
// SetTraps / GetTraps
// ---------------------------------------------------------------------------

func TestSNMP_SetGetTraps(t *testing.T) {
	srv := newTestSNMPServer(&config.SNMPConfig{})
	traps := []config.SNMPTrap{
		{ID: "t1", Target: "127.0.0.1:162", OID: ".1.3.6.1.4.1.9999.1"},
		{ID: "t2", Target: "127.0.0.1:163", OID: ".1.3.6.1.4.1.9999.2"},
	}
	srv.SetTraps(traps)
	got := srv.GetTraps()
	if len(got) != 2 {
		t.Fatalf("want 2 traps, got %d", len(got))
	}
	if got[0].ID != "t1" || got[1].ID != "t2" {
		t.Errorf("unexpected traps: %+v", got)
	}
}

func TestSNMP_GetTraps_IsolatesSlice(t *testing.T) {
	srv := newTestSNMPServer(&config.SNMPConfig{})
	srv.SetTraps([]config.SNMPTrap{{ID: "t1"}})
	got := srv.GetTraps()
	got[0].ID = "mutated"
	if srv.GetTraps()[0].ID != "t1" {
		t.Error("GetTraps should return a copy")
	}
}

// ---------------------------------------------------------------------------
// toInt extras
// ---------------------------------------------------------------------------

func TestToInt_Int64(t *testing.T) {
	if got := toInt(int64(99)); got != 99 {
		t.Errorf("toInt(int64(99)) = %d, want 99", got)
	}
}

func TestToInt_Uint(t *testing.T) {
	if got := toInt(uint(10)); got != 10 {
		t.Errorf("toInt(uint(10)) = %d, want 10", got)
	}
}

func TestToInt_Uint32(t *testing.T) {
	if got := toInt(uint32(123)); got != 123 {
		t.Errorf("toInt(uint32(123)) = %d, want 123", got)
	}
}

func TestToInt_Uint64(t *testing.T) {
	if got := toInt(uint64(456)); got != 456 {
		t.Errorf("toInt(uint64(456)) = %d, want 456", got)
	}
}

// ---------------------------------------------------------------------------
// toUint extras
// ---------------------------------------------------------------------------

func TestToUint_UintDirect(t *testing.T) {
	if got := toUint(uint(5)); got != 5 {
		t.Errorf("toUint(uint(5)) = %d, want 5", got)
	}
}

func TestToUint_Int64Negative(t *testing.T) {
	if got := toUint(int64(-5)); got != 0 {
		t.Errorf("toUint(int64(-5)) = %d, want 0", got)
	}
}

func TestToUint_Float64Positive(t *testing.T) {
	if got := toUint(float64(7.9)); got != 7 {
		t.Errorf("toUint(float64(7.9)) = %d, want 7", got)
	}
}

func TestToUint_Float64Negative(t *testing.T) {
	if got := toUint(float64(-1.0)); got != 0 {
		t.Errorf("toUint(float64(-1.0)) = %d, want 0", got)
	}
}

func TestToUint_Uint32(t *testing.T) {
	if got := toUint(uint32(300)); got != 300 {
		t.Errorf("toUint(uint32(300)) = %d, want 300", got)
	}
}

func TestToUint_Uint64Direct(t *testing.T) {
	if got := toUint(uint64(1000)); got != 1000 {
		t.Errorf("toUint(uint64(1000)) = %d, want 1000", got)
	}
}

func TestToUint_String(t *testing.T) {
	if got := toUint("42"); got != 42 {
		t.Errorf("toUint(\"42\") = %d, want 42", got)
	}
}

// ---------------------------------------------------------------------------
// toUint64 extras
// ---------------------------------------------------------------------------

func TestToUint64_Float64Positive(t *testing.T) {
	if got := toUint64(float64(123.9)); got != 123 {
		t.Errorf("toUint64(float64(123.9)) = %d, want 123", got)
	}
}

func TestToUint64_Float64Negative(t *testing.T) {
	if got := toUint64(float64(-1.0)); got != 0 {
		t.Errorf("toUint64(float64(-1.0)) = %d, want 0", got)
	}
}

func TestToUint64_FallsThrough(t *testing.T) {
	if got := toUint64(int(50)); got != 50 {
		t.Errorf("toUint64 fallthrough = %d, want 50", got)
	}
}

// ---------------------------------------------------------------------------
// authProtocol extras
// ---------------------------------------------------------------------------

func TestAuthProtocol_SHA224(t *testing.T) {
	if got := authProtocol("sha224"); got != gosnmp.SHA224 {
		t.Errorf("authProtocol(sha224) = %v, want SHA224", got)
	}
}

func TestAuthProtocol_SHA384(t *testing.T) {
	if got := authProtocol("sha384"); got != gosnmp.SHA384 {
		t.Errorf("authProtocol(sha384) = %v, want SHA384", got)
	}
}

// ---------------------------------------------------------------------------
// wrapValue extras
// ---------------------------------------------------------------------------

func TestWrapValue_Gauge32(t *testing.T) {
	v, err := wrapValue(gosnmp.Gauge32, 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(uint); !ok || got != 500 {
		t.Errorf("wrapValue(Gauge32, 500) = %v, want uint(500)", v)
	}
}

func TestWrapValue_Counter64(t *testing.T) {
	v, err := wrapValue(gosnmp.Counter64, uint64(1000000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(uint64); !ok || got != 1000000 {
		t.Errorf("wrapValue(Counter64, 1000000) = %v, want uint64(1000000)", v)
	}
}

func TestWrapValue_ObjectIdentifier(t *testing.T) {
	v, err := wrapValue(gosnmp.ObjectIdentifier, ".1.3.6.1.4.1.9999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(string); !ok || got != ".1.3.6.1.4.1.9999" {
		t.Errorf("wrapValue(OID) = %v, want OID string", v)
	}
}

func TestWrapValue_OctetString_Bytes(t *testing.T) {
	v, err := wrapValue(gosnmp.OctetString, []byte("raw bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(string); !ok || got != "raw bytes" {
		t.Errorf("wrapValue(OctetString, []byte) = %v, want 'raw bytes'", v)
	}
}

func TestWrapValue_IPAddress_Invalid(t *testing.T) {
	v, err := wrapValue(gosnmp.IPAddress, "not-an-ip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid IP should fall back to 0.0.0.0.
	if v == nil {
		t.Error("wrapValue(IPAddress, invalid) should return fallback IP, got nil")
	}
}

// ---------------------------------------------------------------------------
// buildUsers
// ---------------------------------------------------------------------------

func TestBuildUsers_Empty(t *testing.T) {
	srv := newTestSNMPServer(&config.SNMPConfig{})
	got := srv.buildUsers()
	if len(got) != 0 {
		t.Errorf("buildUsers with no users should return empty, got %d", len(got))
	}
}

func TestBuildUsers_WithUser(t *testing.T) {
	srv := newTestSNMPServer(&config.SNMPConfig{
		V3Users: []config.SNMPUser{
			{
				Username:       "admin",
				AuthProtocol:   "sha",
				AuthPassphrase: "authpass",
				PrivProtocol:   "aes",
				PrivPassphrase: "privpass",
			},
		},
	})
	got := srv.buildUsers()
	if len(got) != 1 {
		t.Fatalf("want 1 user, got %d", len(got))
	}
	if got[0].UserName != "admin" {
		t.Errorf("unexpected username: %s", got[0].UserName)
	}
	if got[0].AuthenticationProtocol != gosnmp.SHA {
		t.Errorf("unexpected auth protocol: %v", got[0].AuthenticationProtocol)
	}
}
