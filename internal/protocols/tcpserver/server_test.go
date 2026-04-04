// Internal package test so we can access unexported helpers.
package tcpserver

import (
	"testing"
)

func TestMatchTCPPattern_Exact(t *testing.T) {
	if !matchTCPPattern("HELLO", "HELLO", []byte("HELLO")) {
		t.Error("exact string should match")
	}
	if matchTCPPattern("HELLO", "WORLD", []byte("WORLD")) {
		t.Error("wrong exact string should not match")
	}
}

func TestMatchTCPPattern_Regex(t *testing.T) {
	if !matchTCPPattern("re:^PING", "PING 123", []byte("PING 123")) {
		t.Error("regex should match prefix")
	}
	if matchTCPPattern("re:^PING", "PONG", []byte("PONG")) {
		t.Error("regex should not match wrong prefix")
	}
	if matchTCPPattern("re:[invalid", "anything", []byte("anything")) {
		t.Error("invalid regex should not match")
	}
}

func TestMatchTCPPattern_Hex(t *testing.T) {
	if !matchTCPPattern("hex:48454c4c4f", "HELLO", []byte("HELLO")) {
		t.Error("hex pattern should match HELLO bytes")
	}
	if !matchTCPPattern("hex:48 45 4c 4c 4f", "HELLO", []byte("HELLO")) {
		t.Error("hex pattern with spaces should match")
	}
	if matchTCPPattern("hex:48454c4c4f", "WORLD", []byte("WORLD")) {
		t.Error("hex pattern should not match wrong bytes")
	}
	if matchTCPPattern("hex:48454c4c4f4f4f4f4f4f", "HEL", []byte("HEL")) {
		t.Error("hex pattern longer than input should not match")
	}
}

func TestDecodePayload_Plain(t *testing.T) {
	b := decodePayload("hello world")
	if string(b) != "hello world" {
		t.Errorf("want 'hello world', got %q", string(b))
	}
}

func TestDecodePayload_Hex(t *testing.T) {
	b := decodePayload("hex:48454c4c4f")
	if string(b) != "HELLO" {
		t.Errorf("want 'HELLO', got %q", string(b))
	}
}

func TestDecodePayload_HexWithSpaces(t *testing.T) {
	b := decodePayload("hex:48 45 4c 4c 4f")
	if string(b) != "HELLO" {
		t.Errorf("want 'HELLO', got %q", string(b))
	}
}

func TestDecodePayload_InvalidHex(t *testing.T) {
	// Should fall back to treating as plain string.
	b := decodePayload("hex:ZZZZ")
	if string(b) != "hex:ZZZZ" {
		t.Errorf("invalid hex should be returned as-is, got %q", string(b))
	}
}
