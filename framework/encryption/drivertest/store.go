/*
|--------------------------------------------------------------------------
| Keyset Store Contract
|--------------------------------------------------------------------------
|
| The application keeps keysets in its own table. If two servers could
| both create a scope's keyset, or both rotate it, one server's keys
| would be silently overwritten and everything it encrypted lost. The
| contract checks that creating and replacing are atomic.
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
NewStore returns an empty keyset store for one test.
*/
type NewStore func(t *testing.T) encryption.KeysetStore

/*
RunStore runs the contract against stores from the constructor.
*/
func RunStore(t *testing.T, newStore NewStore) {
	t.Helper()

	tenant := encryption.KeysetRef{Scope: encryption.Scope{Kind: "tenant", ID: "1"}, Purpose: encryption.PurposeAEAD}
	other := encryption.KeysetRef{Scope: encryption.Scope{Kind: "tenant", ID: "2"}, Purpose: encryption.PurposeAEAD}

	t.Run("creates a keyset once", func(t *testing.T) {
		t.Parallel()
		store := newStore(t)

		_, err := store.Find(t.Context(), tenant)
		require.ErrorIs(t, err, encryption.ErrKeysetNotFound)

		require.NoError(t, store.Create(t.Context(), tenant, []byte("v1")))
		require.ErrorIs(t, store.Create(t.Context(), tenant, []byte("v2")), encryption.ErrKeysetExists)

		got, err := store.Find(t.Context(), tenant)
		require.NoError(t, err)
		assert.Equal(t, []byte("v1"), got)
	})

	t.Run("replaces only the keyset it read", func(t *testing.T) {
		t.Parallel()
		store := newStore(t)
		require.NoError(t, store.Create(t.Context(), tenant, []byte("v1")))

		require.NoError(t, store.Replace(t.Context(), tenant, []byte("v1"), []byte("v2")))
		require.ErrorIs(t, store.Replace(t.Context(), tenant, []byte("v1"), []byte("v3")), encryption.ErrKeysetChanged)
		require.ErrorIs(t, store.Replace(t.Context(), other, nil, []byte("v1")), encryption.ErrKeysetChanged)

		got, err := store.Find(t.Context(), tenant)
		require.NoError(t, err)
		assert.Equal(t, []byte("v2"), got)
	})

	t.Run("lists every keyset", func(t *testing.T) {
		t.Parallel()
		store := newStore(t)
		require.NoError(t, store.Create(t.Context(), tenant, []byte("a")))
		require.NoError(t, store.Create(t.Context(), other, []byte("b")))

		var refs []encryption.KeysetRef
		for ref, err := range store.All(t.Context()) {
			require.NoError(t, err)
			refs = append(refs, ref)
		}

		assert.ElementsMatch(t, []encryption.KeysetRef{tenant, other}, refs)
	})
}
