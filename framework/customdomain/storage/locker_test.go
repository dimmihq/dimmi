package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain/storage"
)

func TestLocker(t *testing.T) {
	t.Parallel()

	t.Run("locks and unlocks a name", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)

		require.NoError(t, s.Lock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, s.Unlock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, s.Lock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, s.Unlock(t.Context(), "issue_cert_a.com"))
	})

	t.Run("makes a second server wait until the first unlocks", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		first, second := storage.New(db, clock.System()), storage.New(db, clock.System())
		require.NoError(t, first.Lock(t.Context(), "issue_cert_a.com"))

		acquired := make(chan error, 1)
		go func() { acquired <- second.Lock(t.Context(), "issue_cert_a.com") }()

		assert.Never(t, func() bool { return len(acquired) > 0 }, 100*time.Millisecond, 10*time.Millisecond)

		require.NoError(t, first.Unlock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, <-acquired)
		require.NoError(t, second.Unlock(t.Context(), "issue_cert_a.com"))
	})

	t.Run("keeps locks on different names apart", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		first, second := storage.New(db, clock.System()), storage.New(db, clock.System())

		require.NoError(t, first.Lock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, second.Lock(t.Context(), "issue_cert_b.com"))
		require.NoError(t, first.Unlock(t.Context(), "issue_cert_a.com"))
		require.NoError(t, second.Unlock(t.Context(), "issue_cert_b.com"))
	})

	t.Run("stops waiting when the context ends", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		first, second := storage.New(db, clock.System()), storage.New(db, clock.System())
		require.NoError(t, first.Lock(t.Context(), "issue_cert_a.com"))
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		err := second.Lock(ctx, "issue_cert_a.com")

		require.Error(t, err)
		require.NoError(t, first.Unlock(t.Context(), "issue_cert_a.com"))
	})

	t.Run("keeps a held lock after the locking context ends", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		first, second := storage.New(db, clock.System()), storage.New(db, clock.System())
		ctx, cancel := context.WithCancel(t.Context())
		require.NoError(t, first.Lock(ctx, "issue_cert_a.com"))
		cancel()
		waiting, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer stop()

		require.Error(t, second.Lock(waiting, "issue_cert_a.com"))
		require.NoError(t, first.Unlock(t.Context(), "issue_cert_a.com"))
	})

	t.Run("refuses to unlock a name it does not hold", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)

		err := s.Unlock(t.Context(), "issue_cert_a.com")

		assert.ErrorIs(t, err, storage.ErrNotLocked)
	})
}
