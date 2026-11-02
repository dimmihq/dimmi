package bcrypt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/bcrypt"
	"github.com/dimmihq/dimmi/framework/hashing/drivertest"
)

/*
openwallHash is bcrypt's published test vector for the password "U*U".
*/
const openwallHash = "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW"

func newHasher(t *testing.T) hashing.Hasher {
	t.Helper()

	hasher, err := bcrypt.Factory("bcrypt", config.Section{"cost": 4})
	require.NoError(t, err)

	return hasher
}

func TestHasher(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHasher)
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("verifies the openwall test vector", func(t *testing.T) {
		t.Parallel()

		ok, err := newHasher(t).Check(t.Context(), "U*U", openwallHash)

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("asks for a rehash when the cost changes", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		assert.True(t, hasher.NeedsRehash(openwallHash))
	})

	t.Run("refuses passwords longer than bcrypt reads", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)
		long := strings.Repeat("a", 73)

		_, err := hasher.Make(t.Context(), long)
		require.ErrorIs(t, err, hashing.ErrPasswordTooLong)

		hash, err := hasher.Make(t.Context(), long[:72])
		require.NoError(t, err)
		_, err = hasher.Check(t.Context(), long, hash)
		require.ErrorIs(t, err, hashing.ErrPasswordTooLong)
	})

	t.Run("rejects a cost outside bcrypt's range", func(t *testing.T) {
		t.Parallel()

		_, err := bcrypt.New(bcrypt.Options{Cost: 3})
		require.ErrorIs(t, err, bcrypt.ErrInvalidCost)
		_, err = bcrypt.New(bcrypt.Options{Cost: 32})
		require.ErrorIs(t, err, bcrypt.ErrInvalidCost)
		_, err = bcrypt.Factory("x", config.Section{"cost": "high"})
		require.Error(t, err)

		hasher, err := bcrypt.Factory("x", config.Section{})
		require.NoError(t, err)
		assert.True(t, hasher.NeedsRehash(openwallHash))
	})
}
