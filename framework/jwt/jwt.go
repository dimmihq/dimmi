/*
|--------------------------------------------------------------------------
| JSON Web Tokens
|--------------------------------------------------------------------------
|
| Tokens arrive from other people's servers and prove who a visitor is,
| so verification is strict where JWT libraries are lenient. Every key is
| pinned to one algorithm and a token naming another is refused, so an
| attacker cannot downgrade to "none" or sign with a public key as an
| HMAC secret. Base64url must be unpadded and JSON must hold no duplicate
| keys, so no two parsers can read one token differently. Headers that
| point at other keys (jku, x5u, jwk, x5c) are ignored: keys only ever
| come from the caller.
|
| Times are checked against the clock with a small leeway, and a token
| must expire within a bounded lifetime, so a leaked token is not a
| permanent credential.
|
*/

package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Algorithm names a JWS signing algorithm.
*/
type Algorithm string

/*
The supported algorithms: HMAC with SHA-256, RSA PKCS #1 v1.5 with
SHA-256, ECDSA on P-256 with SHA-256, and Ed25519.
*/
const (
	HS256 Algorithm = "HS256"
	RS256 Algorithm = "RS256"
	ES256 Algorithm = "ES256"
	EdDSA Algorithm = "EdDSA"
)

/*
Errors from verifying a token and from building keys.

ErrMalformed covers every encoding problem: padded or non-url base64,
duplicate JSON keys, a payload that is not an object, a crit header, a
wrong typ, or a claim of the wrong type. ErrAlgNotAllowed means the
header names an algorithm the caller did not allow; ErrAlgMismatch that
the key it selects is pinned to another one. ErrMissingExpiry means the
token has no exp. ErrNotYetValid covers nbf and iat in the future.
ErrWeakKey, ErrUnsupportedKey and ErrPrivateKey come from building keys.
*/
var (
	ErrMalformed        = errors.New("jwt: malformed token")
	ErrTooLarge         = errors.New("jwt: token too large")
	ErrAlgNotAllowed    = errors.New("jwt: algorithm not allowed")
	ErrAlgMismatch      = errors.New("jwt: algorithm does not match key")
	ErrUnknownKey       = errors.New("jwt: unknown key")
	ErrSignatureInvalid = errors.New("jwt: invalid signature")
	ErrMissingExpiry    = errors.New("jwt: missing expiry")
	ErrExpired          = errors.New("jwt: token expired")
	ErrNotYetValid      = errors.New("jwt: token not yet valid")
	ErrLifetimeTooLong  = errors.New("jwt: lifetime too long")
	ErrAudience         = errors.New("jwt: audience not accepted")
	ErrIssuer           = errors.New("jwt: issuer not accepted")
	ErrWeakKey          = errors.New("jwt: key too weak")
	ErrUnsupportedKey   = errors.New("jwt: unsupported key")
	ErrPrivateKey       = errors.New("jwt: private key where a public key was expected")
)

/*
Defaults applied to zero Options.
*/
const (
	DefaultLeeway      = time.Minute
	DefaultMaxLifetime = 24 * time.Hour
	DefaultMaxSize     = 8 << 10
)

/*
Options configure Verify.

Algorithms lists the algorithms accepted at all; each key is further
pinned to its own. Audience and Issuer, when set, must appear in the
token. Leeway absorbs clock skew between servers. MaxLifetime bounds the
time from iat, or from now when iat is absent, to exp. MaxSize bounds the
token in bytes. Type, when set, is the required typ header; otherwise
typ may be absent or "JWT". Zero values take the defaults above and the
system clock.
*/
type Options struct {
	Algorithms  []Algorithm
	Audience    string
	Issuer      string
	Leeway      time.Duration
	MaxLifetime time.Duration
	MaxSize     int
	Type        string
	Clock       clock.Clock
}

/*
Claims are a token's claims. Times are zero when absent. Extra holds
every other claim, with numbers as json.Number.
*/
type Claims struct {
	Issuer    string
	Subject   string
	Audience  []string
	ID        string
	Expiry    time.Time
	NotBefore time.Time
	IssuedAt  time.Time
	Extra     map[string]any
}

/*
Key is a key pinned to one algorithm, found by its ID in the kid header.
Its material is a []byte secret for HS256, or an RSA, P-256 or Ed25519
key; signing needs the private key, verifying either.
*/
type Key struct {
	ID        string
	Algorithm Algorithm
	material  any
}

/*
NewKey pins the material to the algorithm. HMAC secrets must be at least
32 bytes and RSA keys at least 2048 bits.
*/
func NewKey(id string, alg Algorithm, material any) (Key, error) {
	if err := checkMaterial(alg, material); err != nil {
		return Key{}, err
	}

	return Key{ID: id, Algorithm: alg, material: material}, nil
}

func checkMaterial(alg Algorithm, material any) error {
	switch alg {
	case HS256:
		return checkSecret(material)
	case RS256:
		return checkRSA(publicOf(material))
	case ES256:
		key, ok := publicOf(material).(*ecdsa.PublicKey)
		if !ok || key.Curve != elliptic.P256() {
			return ErrUnsupportedKey
		}
	case EdDSA:
		if _, ok := publicOf(material).(ed25519.PublicKey); !ok {
			return ErrUnsupportedKey
		}
	default:
		return ErrUnsupportedKey
	}

	return nil
}

func checkSecret(material any) error {
	secret, ok := material.([]byte)
	if !ok {
		return ErrUnsupportedKey
	}

	if len(secret) < minSecretBytes {
		return ErrWeakKey
	}

	return nil
}

func checkRSA(public any) error {
	key, ok := public.(*rsa.PublicKey)
	if !ok {
		return ErrUnsupportedKey
	}

	if key.N.BitLen() < minRSABits {
		return ErrWeakKey
	}

	return nil
}

/*
Minimum key strengths: an HMAC key as long as the hash, and RSA keys of
2048 bits.
*/
const (
	minSecretBytes = 32
	minRSABits     = 2048
)

/*
publicOf returns the public half of a private key and anything else
unchanged.
*/
func publicOf(material any) any {
	if private, ok := material.(interface{ Public() crypto.PublicKey }); ok {
		return private.Public()
	}

	return material
}
