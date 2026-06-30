// Package httpserver — NTLM challenge/response helpers.
package httpserver

import (
	"encoding/base64"
	"strings"
)

// ntlmSignature is the 8-byte NTLM message signature.
const ntlmSignature = "NTLMSSP\x00"

// ntlmType1Prefix is the binary prefix of an NTLM Negotiate (type-1) message.
var ntlmType1Prefix = []byte(ntlmSignature + "\x01")

// ntlmType3Prefix is the binary prefix of an NTLM Authenticate (type-3) message.
var ntlmType3Prefix = []byte(ntlmSignature + "\x03")

// NTLMTokenType classifies an NTLM Authorization header value.
// Returns 1 (Negotiate), 3 (Authenticate), or 0 (not NTLM / unrecognised).
func NTLMTokenType(authHeader string) int {
	const prefix = "NTLM "
	if !strings.HasPrefix(authHeader, prefix) {
		return 0
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(authHeader[len(prefix):]))
	if err != nil || len(b) < 12 {
		return 0
	}
	switch {
	case hasPrefix(b, ntlmType1Prefix):
		return 1
	case hasPrefix(b, ntlmType3Prefix):
		return 3
	}
	return 0
}

func hasPrefix(b, prefix []byte) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == string(prefix)
}

// ntlmChallengeToken returns the base64-encoded NTLM type-2 (Challenge) token
// to include in the WWW-Authenticate header during step 2 of the handshake.
//
// This is a minimal, pre-encoded type-2 message with a static 8-byte challenge.
// Since Mockly accepts any well-formed type-3 response without validating the
// HMAC, the challenge value is irrelevant — any fixed blob works.
//
// The bytes below represent a minimal NTLM Challenge message:
//   - Signature:     NTLMSSP\0      (8 bytes)
//   - MessageType:   \x02\x00\x00\x00 (4 bytes — type 2)
//   - TargetName:    empty          (8 bytes — offset=0, length=0)
//   - NegotiateFlags: 0x00000000    (4 bytes)
//   - ServerChallenge: 8 bytes
//   - Reserved:      8 bytes
func ntlmChallengeToken() string {
	//nolint:gomnd
	msg := []byte{
		// Signature
		'N', 'T', 'L', 'M', 'S', 'S', 'P', 0x00,
		// MessageType = 2
		0x02, 0x00, 0x00, 0x00,
		// TargetNameFields (len=0, maxLen=0, offset=0)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// NegotiateFlags
		0x00, 0x00, 0x00, 0x00,
		// ServerChallenge (static 8-byte value)
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		// Reserved
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	return base64.StdEncoding.EncodeToString(msg)
}
