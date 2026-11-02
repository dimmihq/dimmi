package encryption_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivertest"
	"github.com/dimmihq/dimmi/framework/manager"
)

func TestLocalKEK(t *testing.T) {
	t.Parallel()

	drivertest.RunKEK(t, func(t *testing.T) encryption.KEK {
		t.Helper()

		return localKEK(t)
	})

	t.Run("derives the same kek from the same app key", func(t *testing.T) {
		t.Parallel()
		key := appKey(t)
		first, err := encryption.NewLocalKEK(key)
		require.NoError(t, err)
		second, err := encryption.LocalFactory(key)("local", config.Section{})
		require.NoError(t, err)

		wrapped, err := first.Wrap(t.Context(), []byte("dek"), nil)
		require.NoError(t, err)
		got, err := second.Unwrap(t.Context(), wrapped, nil)
		require.NoError(t, err)
		assert.Equal(t, "dek", string(got))
	})

	t.Run("uses a configured key instead of the app key", func(t *testing.T) {
		t.Parallel()
		other := encryption.GenerateAppKey()
		otherKey, err := encryption.ParseAppKey(other)
		require.NoError(t, err)

		configured, err := encryption.LocalFactory(appKey(t))("old", config.Section{"key": other})
		require.NoError(t, err)
		direct, err := encryption.NewLocalKEK(otherKey)
		require.NoError(t, err)

		wrapped, err := direct.Wrap(t.Context(), []byte("dek"), nil)
		require.NoError(t, err)
		_, err = configured.Unwrap(t.Context(), wrapped, nil)
		require.NoError(t, err)
	})

	t.Run("rejects bad keys and sections", func(t *testing.T) {
		t.Parallel()

		_, err := encryption.NewLocalKEK([]byte("short"))
		require.ErrorIs(t, err, encryption.ErrInvalidAppKey)
		_, err = encryption.LocalFactory(appKey(t))("x", config.Section{"key": "nope"})
		require.ErrorIs(t, err, encryption.ErrInvalidAppKey)
		_, err = encryption.LocalFactory(appKey(t))("x", config.Section{"key": []int{1}})
		require.Error(t, err)
	})
}

func TestManager(t *testing.T) {
	t.Parallel()

	newManager := func(t *testing.T, previous ...string) *encryption.Manager {
		t.Helper()
		m := encryption.NewManager(encryption.Config{
			KEK:      "current",
			Previous: previous,
			KEKs: map[string]config.Section{
				"current": {"driver": "local"},
				"old":     {"driver": "local", "key": encryption.GenerateAppKey()},
			},
		})
		m.Extend("local", encryption.LocalFactory(appKey(t)))

		return m
	}

	t.Run("builds key managers from the configured keks", func(t *testing.T) {
		t.Parallel()
		m := newManager(t, "old")

		keys, err := m.Keys(drivertest.NewPostgresStore(t))
		require.NoError(t, err)

		ciphertext, err := keys.Encrypt(t.Context(), tenant("1"), []byte("x"), nil)
		require.NoError(t, err)
		plaintext, err := keys.Decrypt(t.Context(), ciphertext, nil)
		require.NoError(t, err)
		assert.Equal(t, "x", string(plaintext))

		_, err = m.KEK("old")
		require.NoError(t, err)
	})

	t.Run("reports unknown keks", func(t *testing.T) {
		t.Parallel()

		_, err := newManager(t, "missing").Keys(nil)
		require.ErrorIs(t, err, manager.ErrUnknownInstance)

		m := encryption.NewManager(encryption.Config{KEK: "none"})
		_, err = m.Keys(nil)
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})
}
