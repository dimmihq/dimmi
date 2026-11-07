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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
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
