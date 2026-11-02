package argon2id_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/argon2id"
	"github.com/dimmihq/dimmi/framework/hashing/drivertest"
)

/*
Published and derived Argon2id hashes. The reference vector comes from
the Argon2 reference implementation; the other two reuse its key under
other parameters, which is enough for NeedsRehash.
*/
const (
	reference = "$argon2id$v=19$m=65536,t=2,p=1$c29tZXNhbHQ$CTFhFdXPJO1aFaMaO6Mm5c8y7cJHAph8ArZWb2GRPPc"
	cheapHash = "$argon2id$v=19$m=64,t=1,p=1$c29tZXNhbHQ$CTFhFdXPJO1aFaMaO6Mm5c8y7cJHAph8ArZWb2GRPPc"
	owaspHash = "$argon2id$v=19$m=19456,t=2,p=1$c29tZXNhbHRzYWx0c2FsdA$CTFhFdXPJO1aFaMaO6Mm5c8y7cJHAph8ArZWb2GRPPc"
)

func newHasher(t *testing.T) hashing.Hasher {
	t.Helper()

	hasher, err := argon2id.Factory("argon2id", config.Section{"memory": 64, "iterations": 1, "parallelism": 1})
	require.NoError(t, err)

	return hasher
}

func TestHasher(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHasher)
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("verifies the reference implementation's test vector", func(t *testing.T) {
		t.Parallel()
		ok, err := newHasher(t).Check(t.Context(), "password", reference)

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("writes phc strings with the configured cost", func(t *testing.T) {
		t.Parallel()

		hash, err := newHasher(t).Make(t.Context(), "secret")

		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$"), hash)
	})

	t.Run("asks for a rehash when the cost changes", func(t *testing.T) {
		t.Parallel()
		hash, err := newHasher(t).Make(t.Context(), "secret")
		require.NoError(t, err)

		stronger, err := argon2id.New(argon2id.Options{
			Memory: 128, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
		})
		require.NoError(t, err)

		assert.True(t, stronger.NeedsRehash(hash))
	})

	t.Run("defaults to the owasp cost", func(t *testing.T) {
		t.Parallel()
		hasher, err := argon2id.Factory("argon2id", config.Section{})
		require.NoError(t, err)

		assert.True(t, hasher.NeedsRehash(cheapHash))
		assert.False(t, hasher.NeedsRehash(owaspHash))
	})

	t.Run("rejects parameters argon2id cannot run with", func(t *testing.T) {
		t.Parallel()

		for _, opts := range []argon2id.Options{
			{Memory: 64, Iterations: 0, Parallelism: 1, SaltLength: 16, KeyLength: 32},
			{Memory: 64, Iterations: 1, Parallelism: 0, SaltLength: 16, KeyLength: 32},
			{Memory: 7, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32},
			{Memory: 64, Iterations: 1, Parallelism: 1, SaltLength: 4, KeyLength: 32},
			{Memory: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 8},
		} {
			_, err := argon2id.New(opts)
			require.ErrorIs(t, err, argon2id.ErrInvalidOptions, "%+v", opts)
		}

		_, err := argon2id.Factory("x", config.Section{"memory": "lots"})
		require.Error(t, err)
	})
}
