// Package-level unit tests for snmpserver helpers.
package snmpserver

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

// ---------------------------------------------------------------------------
// snmpType
// ---------------------------------------------------------------------------

func TestSnmpType_Integer(t *testing.T) {
	if got := snmpType("integer"); got != gosnmp.Integer {
		t.Errorf("snmpType(integer) = %v, want %v", got, gosnmp.Integer)
	}
}

func TestSnmpType_IntAlias(t *testing.T) {
	if got := snmpType("int"); got != gosnmp.Integer {
		t.Errorf("snmpType(int) = %v, want %v", got, gosnmp.Integer)
	}
}

func TestSnmpType_Gauge32(t *testing.T) {
	if got := snmpType("gauge32"); got != gosnmp.Gauge32 {
		t.Errorf("snmpType(gauge32) = %v, want %v", got, gosnmp.Gauge32)
	}
}

func TestSnmpType_Counter32(t *testing.T) {
	if got := snmpType("counter32"); got != gosnmp.Counter32 {
		t.Errorf("snmpType(counter32) = %v, want %v", got, gosnmp.Counter32)
	}
}

func TestSnmpType_Counter64(t *testing.T) {
	if got := snmpType("counter64"); got != gosnmp.Counter64 {
		t.Errorf("snmpType(counter64) = %v, want %v", got, gosnmp.Counter64)
	}
}

func TestSnmpType_TimeTicks(t *testing.T) {
	if got := snmpType("timeticks"); got != gosnmp.TimeTicks {
		t.Errorf("snmpType(timeticks) = %v, want %v", got, gosnmp.TimeTicks)
	}
}

func TestSnmpType_IPAddress(t *testing.T) {
	if got := snmpType("ipaddress"); got != gosnmp.IPAddress {
		t.Errorf("snmpType(ipaddress) = %v, want %v", got, gosnmp.IPAddress)
	}
}

func TestSnmpType_ObjectIdentifier(t *testing.T) {
	if got := snmpType("objectidentifier"); got != gosnmp.ObjectIdentifier {
		t.Errorf("snmpType(objectidentifier) = %v, want %v", got, gosnmp.ObjectIdentifier)
	}
}

func TestSnmpType_Default(t *testing.T) {
	// unknown or empty falls through to OctetString
	for _, s := range []string{"string", "octetstring", "", "unknown"} {
		if got := snmpType(s); got != gosnmp.OctetString {
			t.Errorf("snmpType(%q) = %v, want OctetString", s, got)
		}
	}
}

// ---------------------------------------------------------------------------
// wrapValue
// ---------------------------------------------------------------------------

func TestWrapValue_Integer(t *testing.T) {
	v, err := wrapValue(gosnmp.Integer, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(int); !ok || got != 42 {
		t.Errorf("wrapValue(Integer, 42) = %v, want 42", v)
	}
}

func TestWrapValue_OctetString(t *testing.T) {
	v, err := wrapValue(gosnmp.OctetString, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(string); !ok || got != "hello" {
		t.Errorf("wrapValue(OctetString, hello) = %v, want hello", v)
	}
}

func TestWrapValue_Counter32(t *testing.T) {
	v, err := wrapValue(gosnmp.Counter32, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(uint); !ok || got != 100 {
		t.Errorf("wrapValue(Counter32, 100) = %v, want 100", v)
	}
}

func TestWrapValue_TimeTicks(t *testing.T) {
	v, err := wrapValue(gosnmp.TimeTicks, 9876)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, ok := v.(uint32); !ok || got != 9876 {
		t.Errorf("wrapValue(TimeTicks, 9876) = %v, want 9876", v)
	}
}

func TestWrapValue_IPAddress(t *testing.T) {
	v, err := wrapValue(gosnmp.IPAddress, "192.168.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Asn1IPAddressWrap returns a string (the IP's string representation).
	got, ok := v.(string)
	if !ok {
		t.Fatalf("wrapValue(IPAddress) did not return string, got %T", v)
	}
	if got != "192.168.1.1" {
		t.Errorf("IP = %v, want 192.168.1.1", got)
	}
}

// ---------------------------------------------------------------------------
// toInt / toUint / toUint32 / toUint64
// ---------------------------------------------------------------------------

func TestToInt_Int(t *testing.T) {
	if got := toInt(42); got != 42 {
		t.Errorf("toInt(42) = %d, want 42", got)
	}
}

func TestToInt_Float64(t *testing.T) {
	if got := toInt(float64(3.7)); got != 3 {
		t.Errorf("toInt(3.7) = %d, want 3", got)
	}
}

func TestToInt_String(t *testing.T) {
	if got := toInt("99"); got != 99 {
		t.Errorf("toInt(\"99\") = %d, want 99", got)
	}
}

func TestToUint_Negative(t *testing.T) {
	if got := toUint(-1); got != 0 {
		t.Errorf("toUint(-1) = %d, want 0", got)
	}
}

func TestToUint64_Large(t *testing.T) {
	const large = uint64(1 << 40)
	if got := toUint64(large); got != large {
		t.Errorf("toUint64(large) = %d, want %d", got, large)
	}
}

// ---------------------------------------------------------------------------
// authProtocol / privProtocol
// ---------------------------------------------------------------------------

func TestAuthProtocol(t *testing.T) {
	cases := map[string]gosnmp.SnmpV3AuthProtocol{
		"md5":    gosnmp.MD5,
		"sha":    gosnmp.SHA,
		"sha256": gosnmp.SHA256,
		"sha512": gosnmp.SHA512,
		"":       gosnmp.MD5,
	}
	for input, want := range cases {
		if got := authProtocol(input); got != want {
			t.Errorf("authProtocol(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestPrivProtocol(t *testing.T) {
	cases := map[string]gosnmp.SnmpV3PrivProtocol{
		"des":    gosnmp.DES,
		"aes":    gosnmp.AES,
		"aes192": gosnmp.AES192,
		"aes256": gosnmp.AES256,
		"":       gosnmp.DES,
	}
	for input, want := range cases {
		if got := privProtocol(input); got != want {
			t.Errorf("privProtocol(%q) = %v, want %v", input, got, want)
		}
	}
}
