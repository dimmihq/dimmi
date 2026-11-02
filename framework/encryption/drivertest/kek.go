/*
|--------------------------------------------------------------------------
| KEK Contract
|--------------------------------------------------------------------------
|
| Every KEK driver must behave the same, or moving from the local KEK to
| a KMS would change what can be unwrapped. A driver's tests call RunKEK
| with a constructor, and the same suite runs against every driver.
|
|   func TestKEK(t *testing.T) {
|       drivertest.RunKEK(t, newKEK)
|   }
|
*/

package drivertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
)

/*
NewKEK returns a KEK for one test.
*/
type NewKEK func(t *testing.T) encryption.KEK

/*
RunKEK runs the contract against KEKs from the constructor.
*/
func RunKEK(t *testing.T, newKEK NewKEK) {
	t.Helper()

	t.Run("unwraps what it wrapped", func(t *testing.T) {
		t.Parallel()
		kek := newKEK(t)

		wrapped, err := kek.Wrap(t.Context(), []byte("data key"), []byte("tenant/1"))
		require.NoError(t, err)

		got, err := kek.Unwrap(t.Context(), wrapped, []byte("tenant/1"))
		require.NoError(t, err)
		assert.Equal(t, "data key", string(got))
		assert.NotContains(t, string(wrapped), "data key")
	})

	t.Run("wraps the same key differently each time", func(t *testing.T) {
		t.Parallel()
		kek := newKEK(t)

		first, err := kek.Wrap(t.Context(), []byte("data key"), nil)
		require.NoError(t, err)
		second, err := kek.Wrap(t.Context(), []byte("data key"), nil)
		require.NoError(t, err)

		assert.NotEqual(t, first, second)
	})

	t.Run("refuses other associated data or changed bytes", func(t *testing.T) {
		t.Parallel()
		kek := newKEK(t)
		wrapped, err := kek.Wrap(t.Context(), []byte("data key"), []byte("tenant/1"))
		require.NoError(t, err)

		_, err = kek.Unwrap(t.Context(), wrapped, []byte("tenant/2"))
		require.Error(t, err)

		wrapped[len(wrapped)-1] ^= 1
		_, err = kek.Unwrap(t.Context(), wrapped, []byte("tenant/1"))
		require.Error(t, err)
	})
}
