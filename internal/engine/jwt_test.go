package engine_test

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/dever-labs/mockly/internal/engine"
)

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
