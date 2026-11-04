package twofactor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
)

func TestKeySealer(t *testing.T) {
	t.Parallel()

	t.Run("opens a secret only for the row it was sealed for", func(t *testing.T) {
		t.Parallel()
		sealer := newEncrypter(t)
		sealed, err := sealer.Encrypt(t.Context(), []byte("seed"))
		require.NoError(t, err)

		plain, err := sealer.Decrypt(t.Context(), sealed)
		require.NoError(t, err)
		assert.Equal(t, []byte("seed"), plain)

		moved := sealer
		moved.AAD = []byte("another factor")
		_, err = moved.Decrypt(t.Context(), sealed)
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})
}
