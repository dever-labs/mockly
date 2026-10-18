package engine_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/dever-labs/mockly/internal/engine"
	"github.com/dever-labs/mockly/internal/state"
)

// ---------------------------------------------------------------------------
// SetStore
// ---------------------------------------------------------------------------

// TestSetStore_WiresExplicitStore verifies the production wiring path
// (cmd/mockly calls engine.SetStore(store) once at startup with the same
// *state.Store shared by every protocol server): after SetStore, the
// "state"/"state_set" template functions must read/write that exact store
// instance, not some other (e.g. lazily-created fallback) one.
func TestSetStore_WiresExplicitStore(t *testing.T) {
	s := state.New()
	engine.SetStore(s)
	t.Cleanup(func() { engine.SetStore(state.New()) }) // avoid leaking into later tests

	key := "test:setstore:" + randSuffix(t)
	if out := execute(t, `{{state_set "`+key+`" "wired"}}{{state "`+key+`"}}`); out != "wired" {
		t.Fatalf("state round trip via SetStore-wired store: got %q, want %q", out, "wired")
	}

	v, ok := s.Get(key)
	if !ok || v != "wired" {
		t.Errorf("expected the explicit store passed to SetStore to contain the written value, got (%q, %v)", v, ok)
	}
}

// ---------------------------------------------------------------------------
// state / state_set
// ---------------------------------------------------------------------------

func TestFuncs_State_RoundTrip(t *testing.T) {
	key := "test:state:" + randSuffix(t)
	out := execute(t, `{{state_set "`+key+`" "hello"}}{{state "`+key+`"}}`)
	if out != "hello" {
		t.Errorf("state round trip: got %q, want %q", out, "hello")
	}
}

func TestFuncs_State_MissingKeyReturnsEmpty(t *testing.T) {
	out := execute(t, `{{state "test:state:does-not-exist"}}`)
	if out != "" {
		t.Errorf("state for missing key: got %q, want empty string", out)
	}
}

func TestFuncs_StateSetTTL_InvalidTTL(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{state_set_ttl "k" "v" 0}}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("state_set_ttl with ttl<=0: expected error, got nil")
	}
}

func TestFuncs_StateSetTTL_Expires(t *testing.T) {
	key := "test:state-ttl:" + randSuffix(t)
	out := execute(t, `{{state_set_ttl "`+key+`" "hi" 1}}{{state "`+key+`"}}`)
	if out != "hi" {
		t.Fatalf("state_set_ttl: expected immediate read to see value, got %q", out)
	}
	time.Sleep(1200 * time.Millisecond)
	out2 := execute(t, `{{state "`+key+`"}}`)
	if out2 != "" {
		t.Errorf("state_set_ttl: expected value to expire, got %q", out2)
	}
}

func randSuffix(t *testing.T) string {
	t.Helper()
	return execute(t, `{{uuid}}`)
}

// ---------------------------------------------------------------------------
// dict
// ---------------------------------------------------------------------------

func TestFuncs_Dict(t *testing.T) {
	out := execute(t, `{{ $d := dict "sub" "guest" "aud" "my-app" }}{{ $d.sub }}/{{ $d.aud }}`)
	if out != "guest/my-app" {
		t.Errorf("dict: got %q, want %q", out, "guest/my-app")
	}
}

func TestFuncs_Dict_OddArgsError(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{dict "a" "b" "c"}}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("dict with odd args: expected error, got nil")
	}
}

func TestFuncs_Dict_NonStringKeyError(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{dict 1 "b"}}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("dict with non-string key: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// jwt_sign / jwks_json
// ---------------------------------------------------------------------------

func TestFuncs_JWTSign_VerifiesAgainstJWKS(t *testing.T) {
	kid := "test-key-" + randSuffix(t)
	token := execute(t, `{{ jwt_sign (dict "sub" "guest" "nonce" "abc123" "aud" "my-app") "`+kid+`" }}`)
	jwks := execute(t, `{{ jwks_json "`+kid+`" }}`)

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt_sign: expected 3 dot-separated parts, got %d (%q)", len(parts), token)
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	decodeB64URLJSON(t, parts[0], &header)
	if header.Alg != "RS256" || header.Typ != "JWT" || header.Kid != kid {
		t.Errorf("jwt_sign: unexpected header %+v", header)
	}

	var claims struct {
		Sub   string `json:"sub"`
		Nonce string `json:"nonce"`
		Aud   string `json:"aud"`
		Iat   int64  `json:"iat"`
		Exp   int64  `json:"exp"`
	}
	decodeB64URLJSON(t, parts[1], &claims)
	if claims.Sub != "guest" || claims.Nonce != "abc123" || claims.Aud != "my-app" {
		t.Errorf("jwt_sign: unexpected claims %+v", claims)
	}
	if claims.Iat == 0 || claims.Exp == 0 || claims.Exp <= claims.Iat {
		t.Errorf("jwt_sign: expected default iat/exp to be set sanely, got %+v", claims)
	}

	// Verify the signature against the public key served by jwks_json for
	// the same kid — this is the whole point of the feature: a relying
	// party's JWKS-based verifier must accept tokens signed this way.
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(jwks), &doc); err != nil {
		t.Fatalf("jwks_json: invalid JSON: %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0].Kid != kid {
		t.Fatalf("jwks_json: unexpected keys %+v", doc.Keys)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].N)
	if err != nil {
		t.Fatalf("jwks_json: decoding n: %v", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].E)
	if err != nil {
		t.Fatalf("jwks_json: decoding e: %v", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}

	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("jwt_sign: decoding signature: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Errorf("jwt_sign: signature did not verify against jwks_json public key: %v", err)
	}
}

// TestFuncs_JWTSign_AllSupportedAlgs exercises every alg jwt_sign/jwks_json
// support (RS256/384/512, ES256/384/512), signing with an explicit alg
// argument and fully cryptographically verifying the signature against the
// matching jwks_json public key — ensuring none of these algs are merely
// advertised without actually being implemented.
func TestFuncs_JWTSign_AllSupportedAlgs(t *testing.T) {
	algs := []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}
	for _, alg := range algs {
		t.Run(alg, func(t *testing.T) {
			kid := "test-key-" + alg + "-" + randSuffix(t)
			token := execute(t, `{{ jwt_sign (dict "sub" "guest") "`+kid+`" "`+alg+`" }}`)
			jwks := execute(t, `{{ jwks_json "`+kid+`:`+alg+`" }}`)

			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				t.Fatalf("expected 3 dot-separated parts, got %d (%q)", len(parts), token)
			}
			var header struct {
				Alg string `json:"alg"`
				Kid string `json:"kid"`
			}
			decodeB64URLJSON(t, parts[0], &header)
			if header.Alg != alg || header.Kid != kid {
				t.Fatalf("unexpected header %+v", header)
			}

			signingInput := []byte(parts[0] + "." + parts[1])
			sig, err := base64.RawURLEncoding.DecodeString(parts[2])
			if err != nil {
				t.Fatalf("decoding signature: %v", err)
			}

			switch {
			case strings.HasPrefix(alg, "RS"):
				verifyRSA(t, jwks, kid, alg, signingInput, sig)
			case strings.HasPrefix(alg, "ES"):
				verifyECDSA(t, jwks, kid, alg, signingInput, sig)
			}
		})
	}
}

func verifyRSA(t *testing.T, jwks, kid, alg string, signingInput, sig []byte) {
	t.Helper()
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(jwks), &doc); err != nil {
		t.Fatalf("jwks_json: invalid JSON: %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0].Kid != kid {
		t.Fatalf("jwks_json: unexpected keys %+v", doc.Keys)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].N)
	if err != nil {
		t.Fatalf("decoding n: %v", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].E)
	if err != nil {
		t.Fatalf("decoding e: %v", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}

	var digest []byte
	var cryptoHash crypto.Hash
	switch alg {
	case "RS256":
		d := sha256.Sum256(signingInput)
		digest, cryptoHash = d[:], crypto.SHA256
	case "RS384":
		d := sha512.Sum384(signingInput)
		digest, cryptoHash = d[:], crypto.SHA384
	case "RS512":
		d := sha512.Sum512(signingInput)
		digest, cryptoHash = d[:], crypto.SHA512
	}
	if err := rsa.VerifyPKCS1v15(pub, cryptoHash, digest, sig); err != nil {
		t.Errorf("%s: signature did not verify against jwks_json public key: %v", alg, err)
	}
}

func verifyECDSA(t *testing.T, jwks, kid, alg string, signingInput, sig []byte) {
	t.Helper()
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(jwks), &doc); err != nil {
		t.Fatalf("jwks_json: invalid JSON: %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0].Kid != kid {
		t.Fatalf("jwks_json: unexpected keys %+v", doc.Keys)
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].X)
	if err != nil {
		t.Fatalf("decoding x: %v", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(doc.Keys[0].Y)
	if err != nil {
		t.Fatalf("decoding y: %v", err)
	}

	var curve elliptic.Curve
	var digest []byte
	switch alg {
	case "ES256":
		curve = elliptic.P256()
		d := sha256.Sum256(signingInput)
		digest = d[:]
	case "ES384":
		curve = elliptic.P384()
		d := sha512.Sum384(signingInput)
		digest = d[:]
	case "ES512":
		curve = elliptic.P521()
		d := sha512.Sum512(signingInput)
		digest = d[:]
	}
	uncompressed := append([]byte{0x04}, append(xBytes, yBytes...)...)
	pub, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
	if err != nil {
		t.Fatalf("%s: parsing public key from jwks_json x/y: %v", alg, err)
	}

	size := (curve.Params().BitSize + 7) / 8
	if len(sig) != 2*size {
		t.Fatalf("%s: expected signature of %d bytes, got %d", alg, 2*size, len(sig))
	}
	r := new(big.Int).SetBytes(sig[:size])
	s := new(big.Int).SetBytes(sig[size:])
	if !ecdsa.Verify(pub, digest, r, s) {
		t.Errorf("%s: signature did not verify against jwks_json public key", alg)
	}
}

func TestFuncs_JWTSign_KidAlgMismatchError(t *testing.T) {
	kid := "test-key-" + randSuffix(t)
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(
		`{{ jwt_sign (dict "sub" "a") "` + kid + `" "RS256" }}{{ jwt_sign (dict "sub" "b") "` + kid + `" "ES256" }}`,
	)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("jwt_sign with same kid but different alg: expected error, got nil")
	}
}

func TestFuncs_JWTSign_UnsupportedAlgError(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{ jwt_sign (dict "sub" "guest") "k" "HS256" }}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("jwt_sign with unsupported alg: expected error, got nil")
	}
}

func TestFuncs_JWTSign_EmptyKidError(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{ jwt_sign (dict "sub" "guest") "" }}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("jwt_sign with empty kid: expected error, got nil")
	}
}

func TestFuncs_JWKSJSON_NoKidsError(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{ jwks_json }}`)
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err == nil {
		t.Error("jwks_json with no kids: expected error, got nil")
	}
}

func decodeB64URLJSON(t *testing.T, seg string, v interface{}) {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("decoding base64url segment: %v", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decoding JSON segment: %v", err)
	}
}
