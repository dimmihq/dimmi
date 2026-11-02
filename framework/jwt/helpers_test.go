package jwt_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/jwt"
)

/*
testNow is the instant every test verifies at.
*/
func testNow() time.Time {
	return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
}

func secret() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func hmacKey(t *testing.T, id string) jwt.Key {
	t.Helper()

	key, err := jwt.NewKey(id, jwt.HS256, secret())
	require.NoError(t, err)

	return key
}

func rsaKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, bits)
	require.NoError(t, err)

	return key
}

func ecKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	return key
}

func edKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	return key
}

func options() jwt.Options {
	return jwt.Options{Algorithms: []jwt.Algorithm{jwt.HS256}, Clock: clock.NewFake(testNow())}
}

/*
valid returns claims that pass every default check at now.
*/
func valid() jwt.Claims {
	return jwt.Claims{Subject: "user-42", IssuedAt: testNow(), Expiry: testNow().Add(10 * time.Minute)}
}

/*
forge builds an HS256 token from raw header and payload JSON, so tests
can send what no well-behaved library would produce.
*/
func forge(header, payload string) string {
	signingInput := encode(header) + "." + encode(payload)
	mac := hmac.New(sha256.New, secret())
	mac.Write([]byte(signingInput))

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
