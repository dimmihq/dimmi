package testkit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestEd25519(t *testing.T) {
	t.Parallel()

	t.Run("makes a fresh pair with both halves as pem", func(t *testing.T) {
		t.Parallel()
		a, b := testkit.Ed25519(t), testkit.Ed25519(t)
		assert.True(t, strings.HasPrefix(a.PublicPEM, "-----BEGIN PUBLIC KEY-----"))
		assert.True(t, strings.HasPrefix(a.PrivatePEM, "-----BEGIN PRIVATE KEY-----"))
		assert.NotNil(t, a.Private)
		assert.NotEqual(t, a.PublicPEM, b.PublicPEM)
	})

	t.Run("makes a p-256 pair and publishes keys in a key set", func(t *testing.T) {
		t.Parallel()
		ec, ed := testkit.P256(t), testkit.Ed25519(t)
		assert.True(t, strings.HasPrefix(ec.PublicPEM, "-----BEGIN PUBLIC KEY-----"))
		assert.Equal(t, "P-256", ec.JWK["crv"])
		set := string(testkit.JWKS(t, map[string]testkit.KeyPair{"ec": ec, "ed": ed}))
		assert.Contains(t, set, `"kid":"ec"`)
		assert.Contains(t, set, `"crv":"Ed25519"`)
	})

	t.Run("makes browser push keys of the sizes browsers send", func(t *testing.T) {
		t.Parallel()
		k := testkit.BrowserPushKeys(t)
		assert.Len(t, k.P256dh, 87)
		assert.Len(t, k.Auth, 22)
	})
}
