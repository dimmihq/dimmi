package hashing_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/argon2id"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/bcrypt"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
openwallHash is bcrypt's published test vector for the password "U*U".
*/
const openwallHash = "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW"

func newManager() *hashing.Manager {
	m := hashing.NewManager(hashing.Config{
		Default: "argon2id",
		Hashers: map[string]config.Section{
			"argon2id": {"driver": "argon2id", "memory": 64, "iterations": 1},
			"imported": {"driver": "bcrypt", "cost": 4},
		},
	})
	m.Extend("argon2id", argon2id.Factory)
	m.Extend("bcrypt", bcrypt.Factory)

	return m
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("hashes with the default hasher", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		hash, err := m.Make(t.Context(), "secret")
		require.NoError(t, err)

		assert.Contains(t, hash, "$argon2id$")
		ok, err := m.Check(t.Context(), "secret", hash)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.False(t, m.NeedsRehash(hash))
	})

	t.Run("checks an imported hash and asks for a rehash", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		ok, err := m.Check(t.Context(), "U*U", openwallHash)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.True(t, m.NeedsRehash(openwallHash))

		ok, err = m.Check(t.Context(), "wrong", openwallHash)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("normalizes passwords to nfkc", func(t *testing.T) {
		t.Parallel()
		m := newManager()
		composed, decomposed := "café ﬁ", "café fi"

		hash, err := m.Make(t.Context(), composed)
		require.NoError(t, err)

		ok, err := m.Check(t.Context(), decomposed, hash)
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("reports a hash no hasher reads", func(t *testing.T) {
		t.Parallel()
		m := hashing.NewManager(hashing.Config{
			Default: "argon2id",
			Hashers: map[string]config.Section{"argon2id": {"driver": "argon2id", "memory": 64, "iterations": 1}},
		})
		m.Extend("argon2id", argon2id.Factory)

		_, err := m.Check(t.Context(), "secret", "plain")
		require.ErrorIs(t, err, hashing.ErrInvalidHash)
	})

	t.Run("reports a hasher that cannot be built", func(t *testing.T) {
		t.Parallel()
		m := hashing.NewManager(hashing.Config{
			Default: "argon2id",
			Hashers: map[string]config.Section{
				"argon2id": {"driver": "argon2id", "memory": 64, "iterations": 1},
				"broken":   {"driver": "missing"},
			},
		})
		m.Extend("argon2id", argon2id.Factory)

		_, err := m.Check(t.Context(), "U*U", openwallHash)
		require.ErrorIs(t, err, manager.ErrUnknownDriver)

		_, err = m.Hasher("broken")
		require.ErrorIs(t, err, manager.ErrUnknownDriver)
	})

	t.Run("reports a missing default", func(t *testing.T) {
		t.Parallel()
		m := hashing.NewManager(hashing.Config{Default: "none"})

		_, err := m.Make(t.Context(), "secret")
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		_, err = m.Check(t.Context(), "secret", "hash")
		require.ErrorIs(t, err, manager.ErrUnknownInstance)
		assert.False(t, m.NeedsRehash("hash"))
	})
}

func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()

	t.Run("refuses callers beyond the running and waiting slots", func(t *testing.T) {
		t.Parallel()
		m := hashing.NewManager(hashing.Config{
			Default:     "slow",
			Concurrency: 1,
			Queue:       1,
			Hashers:     map[string]config.Section{"slow": {"driver": "argon2id", "memory": 8192, "iterations": 4}},
		})
		m.Extend("argon2id", argon2id.Factory)

		var busy, done atomic.Int32
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				_, err := m.Make(t.Context(), "secret")
				if errors.Is(err, hashing.ErrBusy) {
					busy.Add(1)

					return
				}
				assert.NoError(t, err)
				done.Add(1)
			})
		}
		wg.Wait()

		assert.Positive(t, busy.Load())
		assert.Positive(t, done.Load())
	})

	t.Run("stops waiting when the context ends", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := newManager().Make(ctx, "secret")

		require.ErrorIs(t, err, context.Canceled)
	})
}
