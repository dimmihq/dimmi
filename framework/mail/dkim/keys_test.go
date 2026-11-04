package dkim_test

import (
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail/dkim"
)

func TestKeys(t *testing.T) {
	t.Parallel()

	t.Run("makes a key that signs and publishes its public half", func(t *testing.T) {
		t.Parallel()

		key, err := dkim.GenerateKey()
		require.NoError(t, err)

		der, err := base64.StdEncoding.DecodeString(key.Public)
		require.NoError(t, err)
		_, err = x509.ParsePKIXPublicKey(der)
		require.NoError(t, err)

		public, err := dkim.PublicKey(key.PrivatePEM)
		require.NoError(t, err)
		assert.Equal(t, key.Public, public)

		_, err = dkim.New(dkim.Options{Domain: "acme.test", Selector: "s1", PrivateKey: key.PrivatePEM})
		require.NoError(t, err)
	})

	t.Run("refuses text that is no key", func(t *testing.T) {
		t.Parallel()

		_, err := dkim.PublicKey("not a key")

		require.ErrorIs(t, err, dkim.ErrInvalidKey)
	})
}
