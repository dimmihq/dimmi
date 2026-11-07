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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

/*
KeyPair is a fresh signing key: the PEM a person would upload, the PEM
they must never upload, and the private key to sign with.
*/
type KeyPair struct {
	PublicPEM  string
	PrivatePEM string
	Private    any
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
	}
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
