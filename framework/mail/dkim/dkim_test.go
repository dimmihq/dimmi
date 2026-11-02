package dkim_test

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	maildkim "github.com/dimmihq/dimmi/framework/mail/dkim"
)

const message = "From: Ana <ana@example.com>\r\nTo: bo@example.org\r\nSubject: Hi\r\n" +
	"Message-ID: <m1@example.com>\r\nReceived: by relay\r\n\r\nHello there.\r\n"

func encode(t *testing.T, kind string, der []byte) string {
	t.Helper()

	return string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}))
}

func pkcs8(t *testing.T, key any) string {
	t.Helper()

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	return encode(t, "PRIVATE KEY", der)
}

/*
verify checks the signed message against the public key published as a
DNS record, the way a receiver does.
*/
func verify(t *testing.T, signed []byte, public crypto.PublicKey, algorithm string) *dkim.Verification {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(public)
	require.NoError(t, err)

	if key, ok := public.(ed25519.PublicKey); ok {
		der = key
	}

	record := "v=DKIM1; k=" + algorithm + "; p=" + base64.StdEncoding.EncodeToString(der)
	verifications, err := dkim.VerifyWithOptions(bytes.NewReader(signed), &dkim.VerifyOptions{
		LookupTXT: func(name string) ([]string, error) {
			assert.Equal(t, "sel._domainkey.example.com", name)

			return []string{record}, nil
		},
	})
	require.NoError(t, err)
	require.Len(t, verifications, 1)

	return verifications[0]
}

func TestSigner(t *testing.T) {
	t.Parallel()

	t.Run("signs with an rsa key that receivers verify", func(t *testing.T) {
		t.Parallel()
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		signer, err := maildkim.New(maildkim.Options{
			Domain: "example.com", Selector: "sel",
			PrivateKey: encode(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key)),
		})
		require.NoError(t, err)

		signed, err := signer.Sign([]byte(message))
		require.NoError(t, err)

		result := verify(t, signed, &key.PublicKey, "rsa")
		require.NoError(t, result.Err)
		assert.Equal(t, "example.com", result.Domain)
		assert.Contains(t, result.HeaderKeys, "From")
		assert.NotContains(t, result.HeaderKeys, "Received")
	})

	t.Run("signs with an ed25519 key", func(t *testing.T) {
		t.Parallel()
		public, private, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		signer, err := maildkim.New(maildkim.Options{
			Domain: "example.com", Selector: "sel", PrivateKey: pkcs8(t, private),
		})
		require.NoError(t, err)

		signed, err := signer.Sign([]byte(message))
		require.NoError(t, err)

		require.NoError(t, verify(t, signed, public, "ed25519").Err)
	})

	t.Run("reports a key the signature cannot use", func(t *testing.T) {
		t.Parallel()
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		signer, err := maildkim.New(maildkim.Options{Domain: "example.com", Selector: "sel", PrivateKey: pkcs8(t, key)})
		require.NoError(t, err)

		_, err = signer.Sign([]byte(message))

		require.Error(t, err)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	exchange, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)

	invalid := map[string]maildkim.Options{
		"refuses a missing selector":  {Domain: "example.com", PrivateKey: "x"},
		"refuses text that is no pem": {Domain: "example.com", Selector: "s", PrivateKey: "not a key"},
		"refuses a pem block that is no key": {
			Domain: "example.com", Selector: "s", PrivateKey: encode(t, "PRIVATE KEY", []byte("x")),
		},
		"refuses a key that cannot sign at all": {Domain: "example.com", Selector: "s", PrivateKey: pkcs8(t, exchange)},
	}

	for name, opts := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := maildkim.New(opts)

			require.ErrorIs(t, err, maildkim.ErrInvalidKey)
		})
	}
}
