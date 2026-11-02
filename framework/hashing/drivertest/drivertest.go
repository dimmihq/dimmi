/*
|--------------------------------------------------------------------------
| Hasher Contract
|--------------------------------------------------------------------------
|
| Every hashing driver must behave the same, or switching the default
| algorithm would lock users out. A driver's tests call Run with a
| constructor for hashers at a cheap cost, and the same suite runs
| against every driver.
|
|   func TestHasher(t *testing.T) {
|       drivertest.Run(t, newHasher)
|   }
|
*/

package drivertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
)

/*
NewHasher returns a hasher at a cost low enough for tests.
*/
type NewHasher func(t *testing.T) hashing.Hasher

/*
Run runs the contract against hashers from the constructor.
*/
func Run(t *testing.T, newHasher NewHasher) {
	t.Helper()

	t.Run("passwords", func(t *testing.T) { t.Parallel(); runPasswords(t, newHasher) })
	t.Run("hashes", func(t *testing.T) { t.Parallel(); runHashes(t, newHasher) })
}

func runPasswords(t *testing.T, newHasher NewHasher) {
	t.Helper()

	t.Run("checks the password it hashed", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		hash, err := hasher.Make(t.Context(), "correct horse battery staple")
		require.NoError(t, err)

		ok, err := hasher.Check(t.Context(), "correct horse battery staple", hash)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.NotContains(t, hash, "correct horse")
	})

	t.Run("rejects a wrong password without an error", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)
		hash, err := hasher.Make(t.Context(), "secret")
		require.NoError(t, err)

		for _, wrong := range []string{"Secret", "secret ", "", "secre"} {
			ok, err := hasher.Check(t.Context(), wrong, hash)
			require.NoError(t, err, wrong)
			assert.False(t, ok, wrong)
		}
	})

	t.Run("hashes passwords outside ascii", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		hash, err := hasher.Make(t.Context(), "пароль-密码-🔑")
		require.NoError(t, err)

		ok, err := hasher.Check(t.Context(), "пароль-密码-🔑", hash)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}

func runHashes(t *testing.T, newHasher NewHasher) {
	t.Helper()

	t.Run("salts every hash", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		first, err := hasher.Make(t.Context(), "same")
		require.NoError(t, err)
		second, err := hasher.Make(t.Context(), "same")
		require.NoError(t, err)

		assert.NotEqual(t, first, second)
	})

	t.Run("reports a hash it cannot read", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		for _, hash := range []string{"", "plain", "$unknown$v=1$abc"} {
			_, err := hasher.Check(t.Context(), "secret", hash)
			require.ErrorIs(t, err, hashing.ErrInvalidHash, hash)
			assert.True(t, hasher.NeedsRehash(hash), hash)
		}
	})

	t.Run("needs no rehash for its own fresh hash", func(t *testing.T) {
		t.Parallel()
		hasher := newHasher(t)

		hash, err := hasher.Make(t.Context(), "secret")
		require.NoError(t, err)

		assert.False(t, hasher.NeedsRehash(hash))
	})
}
