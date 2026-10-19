package engine

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"math/big"
	"strings"
	"sync"
	"time"
)

// rsaKeyBits is the size of every lazily-generated RSA signing key. 2048
// bits is the minimum size most JWT/JOSE libraries (and the OIDC spec)
// treat as acceptable, and is plenty fast to generate on first use of a
// given kid.
const rsaKeyBits = 2048

// signingKey is a lazily-generated key pair bound to one kid and one JWS
// alg. Exactly one of rsaKey/ecKey is set, matching alg's key family.
type signingKey struct {
	alg    string
	rsaKey *rsa.PrivateKey
	ecKey  *ecdsa.PrivateKey
}

var (
	keysMu sync.Mutex
	keys   = map[string]*signingKey{}
)

// algInfo describes the hash and curve (if any) behind a supported JWS alg.
type algInfo struct {
	kty       string // "RSA" or "EC"
	crv       string // set for EC algs
	curve     elliptic.Curve
	newHash   func() hash.Hash
	cryptoSum crypto.Hash
}

var supportedAlgs = map[string]algInfo{
	"RS256": {kty: "RSA", newHash: sha256.New, cryptoSum: crypto.SHA256},
	"RS384": {kty: "RSA", newHash: sha512.New384, cryptoSum: crypto.SHA384},
	"RS512": {kty: "RSA", newHash: sha512.New, cryptoSum: crypto.SHA512},
	"ES256": {kty: "EC", crv: "P-256", curve: elliptic.P256(), newHash: sha256.New, cryptoSum: crypto.SHA256},
	"ES384": {kty: "EC", crv: "P-384", curve: elliptic.P384(), newHash: sha512.New384, cryptoSum: crypto.SHA384},
	"ES512": {kty: "EC", crv: "P-521", curve: elliptic.P521(), newHash: sha512.New, cryptoSum: crypto.SHA512},
}

// supportedAlgNames lists every alg jwt_sign/jwks_json accept, in a stable
// order, for use in error messages.
func supportedAlgNames() []string {
	return []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}
}

// keyForKID returns the signing key registered under kid, generating a
// fresh key pair for alg the first time kid is seen and reusing it for the
// lifetime of the process thereafter. A kid is permanently bound to the
// first alg it's requested with (per RFC 7517 — a "kid" identifies one
// specific key, which has exactly one alg); a later call that names a
// different alg for the same kid is a preset-authoring error and returns
// one, surfacing the mismatch immediately rather than silently signing (or
// serving) the wrong key type.
func keyForKID(kid, alg string) (*signingKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("key id must not be empty")
	}
	if alg == "" {
		alg = "RS256"
	}
	info, ok := supportedAlgs[alg]
	if !ok {
		return nil, fmt.Errorf("unsupported alg %q (supported: %s)", alg, strings.Join(supportedAlgNames(), ", "))
	}

	keysMu.Lock()
	defer keysMu.Unlock()
	if k, ok := keys[kid]; ok {
		if k.alg != alg {
			return nil, fmt.Errorf("key id %q is already registered for alg %q, got %q — use a different kid per alg", kid, k.alg, alg)
		}
		return k, nil
	}

	k := &signingKey{alg: alg}
	var err error
	switch info.kty {
	case "RSA":
		k.rsaKey, err = rsa.GenerateKey(rand.Reader, rsaKeyBits)
	case "EC":
		k.ecKey, err = ecdsa.GenerateKey(info.curve, rand.Reader)
	}
	if err != nil {
		return nil, fmt.Errorf("generating %s signing key %q: %w", alg, kid, err)
	}
	keys[kid] = k
	return k, nil
}

func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// fnDict builds a map[string]interface{} from alternating key/value
// arguments — e.g. dict "sub" "guest" "aud" "my-app" → {"sub":"guest","aud":"my-app"}.
// Mirrors the "dict" helper found in Sprig/Helm templates; used to build a
// claims map to pass to jwt_sign.
func fnDict(pairs ...interface{}) (map[string]interface{}, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: requires an even number of arguments (key/value pairs), got %d", len(pairs))
	}
	m := make(map[string]interface{}, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		k, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: argument %d (%v) must be a string key", i+1, pairs[i])
		}
		m[k] = pairs[i+1]
	}
	return m, nil
}

// fnJWTSign signs claims as a compact JWT using the key registered under
// kid (see keyForKID — generated lazily on first use; expose its public
// half to clients via jwks_json so signatures can be verified).
//
// alg is optional and defaults to "RS256" when omitted; supported values are
// RS256, RS384, RS512, ES256, ES384 and ES512.
//
// "iat" and "exp" are filled in with sane defaults (now / now+1h) whenever
// the caller's claims map doesn't already set them, so a minimal
// `dict "sub" "guest"` still produces a token most OIDC/JWT libraries accept.
func fnJWTSign(claims map[string]interface{}, kid string, alg ...string) (string, error) {
	if len(alg) > 1 {
		return "", fmt.Errorf("jwt_sign: at most one alg argument is accepted, got %d", len(alg))
	}
	a := ""
	if len(alg) == 1 {
		a = alg[0]
	}
	key, err := keyForKID(kid, a)
	if err != nil {
		return "", fmt.Errorf("jwt_sign: %w", err)
	}

	now := time.Now().UTC()
	full := make(map[string]interface{}, len(claims)+2)
	for k, v := range claims {
		full[k] = v
	}
	if _, ok := full["iat"]; !ok {
		full["iat"] = now.Unix()
	}
	if _, ok := full["exp"]; !ok {
		full["exp"] = now.Add(time.Hour).Unix()
	}

	header := map[string]interface{}{"alg": key.alg, "typ": "JWT", "kid": kid}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("jwt_sign: encoding header: %w", err)
	}
	claimsJSON, err := json.Marshal(full)
	if err != nil {
		return "", fmt.Errorf("jwt_sign: encoding claims: %w", err)
	}

	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	sig, err := signWith(key, []byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("jwt_sign: signing: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}

// signWith produces the raw JWS signature bytes for signingInput using key,
// per RFC 7518 §3 (RSASSA-PKCS1-v1_5 for RSxxx, ECDSA for ESxxx — the latter
// as the fixed-width R||S concatenation JWS requires, not ASN.1 DER).
func signWith(key *signingKey, signingInput []byte) ([]byte, error) {
	info := supportedAlgs[key.alg]
	h := info.newHash()
	h.Write(signingInput)
	digest := h.Sum(nil)

	switch {
	case key.rsaKey != nil:
		return rsa.SignPKCS1v15(rand.Reader, key.rsaKey, info.cryptoSum, digest)
	case key.ecKey != nil:
		r, s, err := ecdsa.Sign(rand.Reader, key.ecKey, digest)
		if err != nil {
			return nil, err
		}
		size := (key.ecKey.Curve.Params().BitSize + 7) / 8
		out := make([]byte, 2*size)
		r.FillBytes(out[:size])
		s.FillBytes(out[size:])
		return out, nil
	default:
		return nil, fmt.Errorf("key %q has no key material", key.alg)
	}
}

// ecPublicXY returns the raw, fixed-width X and Y coordinates of pub, using
// the non-deprecated PublicKey.Bytes() uncompressed-point encoding (SEC 1
// §2.3.3: 0x04 || X || Y) rather than reading pub.X/pub.Y directly.
func ecPublicXY(pub ecdsa.PublicKey) (x, y []byte, err error) {
	raw, err := pub.Bytes()
	if err != nil {
		return nil, nil, err
	}
	size := (len(raw) - 1) / 2
	return raw[1 : 1+size], raw[1+size:], nil
}

// jwkKey is a single RFC 7517/7518 JSON Web Key, covering both the RSA and
// EC fields a jwt_sign-signed token might need to be verified.
type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// fnJWKSJSON returns a JWKS ({"keys": [...]}) document exposing the public
// half of one or more signing keys, so an OIDC preset's jwks_uri endpoint
// can serve keys that actually verify the JWTs that same preset's token
// endpoint signs via jwt_sign.
//
// Each argument is either a bare key id (defaulting to alg "RS256") or
// "kid:ALG" to request a specific algorithm's key pair, e.g.
// jwks_json "mockly-key-1" "mockly-ec-key-1:ES256".
func fnJWKSJSON(kids ...string) (string, error) {
	if len(kids) == 0 {
		return "", fmt.Errorf("jwks_json: at least one key id is required")
	}
	out := make([]jwkKey, 0, len(kids))
	for _, spec := range kids {
		kid, alg := spec, ""
		if idx := strings.LastIndex(spec, ":"); idx >= 0 {
			kid, alg = spec[:idx], spec[idx+1:]
		}
		key, err := keyForKID(kid, alg)
		if err != nil {
			return "", fmt.Errorf("jwks_json: %w", err)
		}
		switch {
		case key.rsaKey != nil:
			pub := key.rsaKey.PublicKey
			out = append(out, jwkKey{
				Kid: kid, Kty: "RSA", Use: "sig", Alg: key.alg,
				N: b64url(pub.N.Bytes()),
				E: b64url(big.NewInt(int64(pub.E)).Bytes()),
			})
		case key.ecKey != nil:
			xBytes, yBytes, err := ecPublicXY(key.ecKey.PublicKey)
			if err != nil {
				return "", fmt.Errorf("jwks_json: encoding EC public key %q: %w", kid, err)
			}
			out = append(out, jwkKey{
				Kid: kid, Kty: "EC", Use: "sig", Alg: key.alg,
				Crv: supportedAlgs[key.alg].crv,
				X:   b64url(xBytes),
				Y:   b64url(yBytes),
			})
		}
	}
	data, err := json.Marshal(map[string]interface{}{"keys": out})
	if err != nil {
		return "", fmt.Errorf("jwks_json: encoding: %w", err)
	}
	return string(data), nil
}
