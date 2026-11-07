/*
|--------------------------------------------------------------------------
| Test Signing Keys
|--------------------------------------------------------------------------
|
| Tests of token verification sign with keys a host's server would hold.
| Each call makes a fresh pair, so no private key is ever checked in, and
| hands back the PEM text a person would paste beside the key to sign
| with.
|
*/

package testkit

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"maps"
	"testing"
)

/*
KeyPair is a fresh signing key: the PEM a person would upload, the PEM
they must never upload, the private key to sign with, and the public
key as an auth provider would publish it in a key set.
*/
type KeyPair struct {
	PublicPEM  string
	PrivatePEM string
	Private    any
	JWK        map[string]string
}

/*
Ed25519 returns a fresh Ed25519 key pair, failing the test when one
cannot be made.
*/
func Ed25519(tb testing.TB) KeyPair {
	tb.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("generate ed25519 key: %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		tb.Fatalf("encode ed25519 public key: %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		tb.Fatalf("encode ed25519 private key: %v", err)
	}

	return KeyPair{
		PublicPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		PrivatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		Private:    private,
		JWK: map[string]string{
			"kty": "OKP", "crv": "Ed25519", "alg": "EdDSA", "use": "sig",
			"x": base64.RawURLEncoding.EncodeToString(public),
		},
	}
}

/*
P256 returns a fresh ECDSA P-256 key pair, as auth providers sign ES256
tokens with, failing the test when one cannot be made.
*/
func P256(tb testing.TB) KeyPair {
	tb.Helper()

	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatalf("generate p-256 key: %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil {
		tb.Fatalf("encode p-256 public key: %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		tb.Fatalf("encode p-256 private key: %v", err)
	}

	point, err := private.PublicKey.Bytes()
	if err != nil {
		tb.Fatalf("encode p-256 point: %v", err)
	}

	encode := base64.RawURLEncoding.EncodeToString

	return KeyPair{
		PublicPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		PrivatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		Private:    private,
		JWK: map[string]string{
			"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig",
			"x": encode(point[1:33]), "y": encode(point[33:]),
		},
	}
}

/*
JWKS returns the key set document publishing the pairs' public keys,
each under its key id, as an auth provider serves it.
*/
func JWKS(tb testing.TB, keys map[string]KeyPair) []byte {
	tb.Helper()

	set := struct {
		Keys []map[string]string `json:"keys"`
	}{Keys: []map[string]string{}}

	for kid, pair := range keys {
		jwk := map[string]string{"kid": kid}
		maps.Copy(jwk, pair.JWK)

		set.Keys = append(set.Keys, jwk)
	}

	out, err := json.Marshal(set)
	if err != nil {
		tb.Fatalf("encode jwks: %v", err)
	}

	return out
}

/*
PushKeys are the keys a browser's push subscription carries, base64url
as the browser gives them: its P-256 public key and its auth secret.
*/
type PushKeys struct {
	P256dh string
	Auth   string
}

/*
BrowserPushKeys returns fresh push subscription keys, failing the test
when they cannot be made.
*/
func BrowserPushKeys(tb testing.TB) PushKeys {
	tb.Helper()

	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("generate push key: %v", err)
	}

	secret := make([]byte, 16)
	if _, err := rand.Read(secret); err != nil {
		tb.Fatalf("generate push secret: %v", err)
	}

	encode := base64.RawURLEncoding.EncodeToString

	return PushKeys{P256dh: encode(key.PublicKey().Bytes()), Auth: encode(secret)}
}
