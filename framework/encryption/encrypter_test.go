package encryption_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
)

func TestAppKey(t *testing.T) {
	t.Parallel()

	t.Run("generates distinct keys that parse back", func(t *testing.T) {
		t.Parallel()
		first, second := encryption.GenerateAppKey(), encryption.GenerateAppKey()

		assert.True(t, strings.HasPrefix(first, "base64:"))
		assert.NotEqual(t, first, second)
		key, err := encryption.ParseAppKey(first)
		require.NoError(t, err)
		assert.Len(t, key, 32)
	})

	t.Run("rejects malformed keys", func(t *testing.T) {
		t.Parallel()

		malformed := []string{"", "c2hvcnQ=", "base64:c2hvcnQ=", "base64:!!!", "base64:" + strings.Repeat("A", 44)}
		for _, s := range malformed {
			_, err := encryption.ParseAppKey(s)
			require.ErrorIs(t, err, encryption.ErrInvalidAppKey, s)
		}
	})

	t.Run("derives unrelated keys per purpose", func(t *testing.T) {
		t.Parallel()
		key := appKey(t)

		a, err := encryption.DeriveKey(key, "a")
		require.NoError(t, err)
		b, err := encryption.DeriveKey(key, "b")
		require.NoError(t, err)
		again, err := encryption.DeriveKey(key, "a")
		require.NoError(t, err)

		assert.NotEqual(t, a, b)
		assert.Equal(t, a, again)
		_, err = encryption.DeriveKey([]byte("short"), "a")
		require.ErrorIs(t, err, encryption.ErrInvalidAppKey)
	})
}

func TestEncrypter(t *testing.T) {
	t.Parallel()

	t.Run("round trips bytes and strings", func(t *testing.T) {
		t.Parallel()
		enc, err := encryption.NewEncrypter(appKey(t))
		require.NoError(t, err)

		ciphertext, err := enc.Encrypt([]byte("secret"))
		require.NoError(t, err)
		plaintext, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, "secret", string(plaintext))

		token, err := enc.EncryptString("state=1&next=/home")
		require.NoError(t, err)
		assert.NotContains(t, token, "=")
		value, err := enc.DecryptString(token)
		require.NoError(t, err)
		assert.Equal(t, "state=1&next=/home", value)
	})

	t.Run("keeps decrypting with previous keys after rotation", func(t *testing.T) {
		t.Parallel()
		old, current := appKey(t), appKey(t)
		before, err := encryption.NewEncrypter(old)
		require.NoError(t, err)
		token, err := before.EncryptString("cookie")
		require.NoError(t, err)

		after, err := encryption.NewEncrypter(current, old)
		require.NoError(t, err)
		value, err := after.DecryptString(token)
		require.NoError(t, err)
		assert.Equal(t, "cookie", value)

		fresh, err := encryption.NewEncrypter(current)
		require.NoError(t, err)
		_, err = fresh.DecryptString(token)
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("refuses changed or foreign ciphertext", func(t *testing.T) {
		t.Parallel()
		enc, err := encryption.NewEncrypter(appKey(t))
		require.NoError(t, err)
		ciphertext, err := enc.Encrypt([]byte("secret"))
		require.NoError(t, err)

		ciphertext[len(ciphertext)-1] ^= 1
		_, err = enc.Decrypt(ciphertext)
		require.ErrorIs(t, err, encryption.ErrDecrypt)
		_, err = enc.DecryptString("not base64!")
		require.ErrorIs(t, err, encryption.ErrDecrypt)
		_, err = enc.DecryptString("")
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("requires valid keys", func(t *testing.T) {
		t.Parallel()

		_, err := encryption.NewEncrypter(appKey(t), []byte("short"))

		require.ErrorIs(t, err, encryption.ErrInvalidAppKey)
	})
}
