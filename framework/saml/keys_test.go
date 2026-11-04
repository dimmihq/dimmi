package saml_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/saml"
)

func TestKeyPair(t *testing.T) {
	t.Parallel()

	t.Run("makes a self-signed certificate for the entity id that lasts ten years", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

		pair, err := saml.NewKeyPair("https://sp.example/metadata", now)
		require.NoError(t, err)

		certificate, err := saml.ParseCertificate(pair.Certificate)
		require.NoError(t, err)
		assert.Equal(t, "https://sp.example/metadata", certificate.Subject.CommonName)
		assert.True(t, certificate.NotAfter.After(now.AddDate(9, 11, 0)))
		assert.Contains(t, pair.PrivateKey, "RSA PRIVATE KEY")
	})

	t.Run("reads a certificate without its pem armor and refuses nonsense", func(t *testing.T) {
		t.Parallel()
		pair, err := saml.NewKeyPair("sp", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		lines := pair.Certificate
		body := lines[len("-----BEGIN CERTIFICATE-----\n") : len(lines)-len("-----END CERTIFICATE-----\n")]
		_, err = saml.ParseCertificate(body)
		require.NoError(t, err)

		_, err = saml.ParseCertificate("nonsense")
		require.ErrorIs(t, err, saml.ErrInvalidKey)
		_, err = saml.ParseCertificate("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n")
		require.ErrorIs(t, err, saml.ErrInvalidKey)
	})
}
