package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/support/keysets"
	"github.com/dimmihq/dimmi/tests"
)

func TestEncryptionKeys(t *testing.T) {
	t.Parallel()

	system := encryption.Scope{Kind: "system"}

	t.Run("keeps one wrapped keyset per scope and purpose, created on first use", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		sealed, err := tc.App.Keys.Encrypt(t.Context(), system, []byte("seed"), []byte("row 1"))
		require.NoError(t, err)
		_, err = tc.App.Keys.Encrypt(t.Context(), system, []byte("other"), []byte("row 2"))
		require.NoError(t, err)

		tc.Table("encryption_keys").AssertCount(1).
			AssertHas(testkit.Row{"scope_kind": "system", "scope_id": "", "purpose": "aead"})
		plain, err := tc.App.Keys.Decrypt(t.Context(), sealed, []byte("row 1"))
		require.NoError(t, err)
		assert.Equal(t, []byte("seed"), plain)
		_, err = tc.App.Keys.Decrypt(t.Context(), sealed, []byte("row 2"))
		require.ErrorIs(t, err, encryption.ErrDecrypt)
	})

	t.Run("rotates in place and still opens what older keys sealed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sealed, err := tc.App.Keys.Encrypt(t.Context(), system, []byte("seed"), nil)
		require.NoError(t, err)

		require.NoError(t, tc.App.Keys.Rotate(t.Context(), system))

		plain, err := tc.App.Keys.Decrypt(t.Context(), sealed, nil)
		require.NoError(t, err)
		assert.Equal(t, []byte("seed"), plain)
		tc.Table("encryption_keys").AssertCount(1, testkit.Row{"scope_kind": "system", "purpose": "aead"})
	})

	t.Run("never overwrites a keyset another server stored or changed first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		store := &keysets.Store{DB: tc.App.DB}
		ref := encryption.KeysetRef{Scope: encryption.Scope{Kind: "workspace", ID: "1"}, Purpose: encryption.PurposeMAC}

		_, err := store.Find(t.Context(), ref)
		require.ErrorIs(t, err, encryption.ErrKeysetNotFound)
		require.NoError(t, store.Create(t.Context(), ref, []byte("first")))
		require.ErrorIs(t, store.Create(t.Context(), ref, []byte("second")), encryption.ErrKeysetExists)
		require.ErrorIs(t, store.Replace(t.Context(), ref, []byte("second"), []byte("third")),
			encryption.ErrKeysetChanged)
		require.NoError(t, store.Replace(t.Context(), ref, []byte("first"), []byte("third")))

		wrapped, err := store.Find(t.Context(), ref)
		require.NoError(t, err)
		assert.Equal(t, []byte("third"), wrapped)

		var refs []encryption.KeysetRef
		for listed, err := range store.All(t.Context()) {
			require.NoError(t, err)
			refs = append(refs, listed)
		}
		assert.Equal(t, []encryption.KeysetRef{ref}, refs)
	})

	t.Run("reports a store it cannot read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		store := &keysets.Store{DB: tc.App.DB}
		ref := encryption.KeysetRef{Scope: encryption.Scope{Kind: "system"}, Purpose: encryption.PurposeAEAD}
		tc.BreakTable("encryption_keys")

		_, err := store.Find(t.Context(), ref)
		require.Error(t, err)
		require.Error(t, store.Create(t.Context(), ref, []byte("v")))
		require.Error(t, store.Replace(t.Context(), ref, []byte("v"), []byte("w")))
		for _, err := range store.All(t.Context()) {
			require.Error(t, err)
		}
	})
}
