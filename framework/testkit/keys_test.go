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

	t.Run("makes browser push keys of the sizes browsers send", func(t *testing.T) {
		t.Parallel()
		k := testkit.BrowserPushKeys(t)
		assert.Len(t, k.P256dh, 87)
		assert.Len(t, k.Auth, 22)
	})
}
