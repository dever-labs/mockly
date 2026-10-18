package engine

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// rsaKeyBits is the size of every lazily-generated signing key. 2048 bits is
// the minimum RSA key size most JWT/JOSE libraries (and the OIDC spec) treat
// as acceptable, and is plenty fast to generate on first use of a given kid.
const rsaKeyBits = 2048

var (
	rsaKeysMu sync.Mutex
	rsaKeys   = map[string]*rsa.PrivateKey{}
)

// rsaKeyForKID returns the RSA private key registered under kid, generating
// a fresh key pair the first time kid is seen and reusing it for the
// lifetime of the process thereafter. This is what keeps a preset's
// {{jwks_json "kid"}} endpoint and its {{jwt_sign ... "kid"}}-signed tokens
// consistent with each other: both calls resolve the same kid to the same
// key pair.
func rsaKeyForKID(kid string) (*rsa.PrivateKey, error) {
	rsaKeysMu.Lock()
	defer rsaKeysMu.Unlock()
	if k, ok := rsaKeys[kid]; ok {
		return k, nil
	}
	k, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return nil, fmt.Errorf("jwt: generating signing key %q: %w", kid, err)
	}
	rsaKeys[kid] = k
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

// fnJWTSign signs claims as a compact RS256 JWT using the key registered
// under kid (see rsaKeyForKID — generated lazily on first use; expose its
// public half to clients via jwks_json so signatures can be verified).
//
// "iat" and "exp" are filled in with sane defaults (now / now+1h) whenever
// the caller's claims map doesn't already set them, so a minimal
// `dict "sub" "guest"` still produces a token most OIDC/JWT libraries accept.
func fnJWTSign(claims map[string]interface{}, kid string) (string, error) {
	if kid == "" {
		return "", fmt.Errorf("jwt_sign: key id must not be empty")
	}
	key, err := rsaKeyForKID(kid)
	if err != nil {
		return "", err
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

	header := map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": kid}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("jwt_sign: encoding header: %w", err)
	}
	claimsJSON, err := json.Marshal(full)
	if err != nil {
		return "", fmt.Errorf("jwt_sign: encoding claims: %w", err)
	}

	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("jwt_sign: signing: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}

// jwkRSA is a single RFC 7517 JSON Web Key for an RSA public key.
type jwkRSA struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fnJWKSJSON returns a JWKS ({"keys": [...]}) document exposing the public
// half of every kid's signing key (generated lazily, same as jwt_sign), so
// an OIDC preset's jwks_uri endpoint can serve keys that actually verify the
// JWTs that same preset's token endpoint signs via jwt_sign.
func fnJWKSJSON(kids ...string) (string, error) {
	if len(kids) == 0 {
		return "", fmt.Errorf("jwks_json: at least one key id is required")
	}
	keys := make([]jwkRSA, 0, len(kids))
	for _, kid := range kids {
		key, err := rsaKeyForKID(kid)
		if err != nil {
			return "", err
		}
		pub := key.PublicKey
		keys = append(keys, jwkRSA{
			Kid: kid,
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			N:   b64url(pub.N.Bytes()),
			E:   b64url(big.NewInt(int64(pub.E)).Bytes()),
		})
	}
	out, err := json.Marshal(map[string]interface{}{"keys": keys})
	if err != nil {
		return "", fmt.Errorf("jwks_json: encoding: %w", err)
	}
	return string(out), nil
}
