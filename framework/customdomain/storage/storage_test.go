package storage_test

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain/storage"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
openDB returns a pool on a fresh database migrated with the storage
table.
*/
func openDB(t *testing.T) *database.DB {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Framework: storage.Migrations()}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: url, MaxConns: 6, PoolMode: database.PoolModeSession})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
newStorage returns storage on a fresh database with a frozen clock.
*/
func newStorage(t *testing.T) (*storage.Storage, *clock.Fake) {
	t.Helper()

	clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))

	return storage.New(openDB(t), clk), clk
}

/*
seed stores each key with its own name as the value.
*/
func seed(t *testing.T, s *storage.Storage, keys ...string) {
	t.Helper()

	for _, key := range keys {
		require.NoError(t, s.Store(t.Context(), key, []byte(key)))
	}
}

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("pass the migration lint", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, migrate.Lint(storage.Migrations()))
	})
}

func TestStorage(t *testing.T) {
	t.Parallel()

	t.Run("stores, replaces and loads a value", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)

		require.NoError(t, s.Store(t.Context(), "acme/account.json", []byte("one")))
		require.NoError(t, s.Store(t.Context(), "/acme/account.json/", []byte("two")))
		value, err := s.Load(t.Context(), "acme/account.json")

		require.NoError(t, err)
		assert.Equal(t, []byte("two"), value)
	})

	t.Run("reports a missing key as not existing", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)

		_, err := s.Load(t.Context(), "missing")

		require.ErrorIs(t, err, fs.ErrNotExist)
		assert.False(t, s.Exists(t.Context(), "missing"))
	})

	t.Run("treats a key prefixing others as an existing directory", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)
		seed(t, s, "certs/example.com/example.com.crt")

		assert.True(t, s.Exists(t.Context(), "certs/example.com/example.com.crt"))
		assert.True(t, s.Exists(t.Context(), "certs"))
		assert.False(t, s.Exists(t.Context(), "cert"))
	})

	t.Run("deletes a key and everything under it", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)
		seed(t, s, "certs/a.com/a.crt", "certs/a.com/a.key", "certs/a.community/x.crt", "certs/b.com/b.crt")

		require.NoError(t, s.Delete(t.Context(), "certs/a.com"))
		require.NoError(t, s.Delete(t.Context(), "certs/a.com"))

		assert.False(t, s.Exists(t.Context(), "certs/a.com"))
		assert.True(t, s.Exists(t.Context(), "certs/a.community/x.crt"))
		assert.True(t, s.Exists(t.Context(), "certs/b.com/b.crt"))
	})

	t.Run("lists the direct entries of a directory", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)
		seed(t, s, "certs/a.com/a.crt", "certs/a.com/a.key", "certs/b.com/b.crt", "certs/readme", "other/x")

		entries, err := s.List(t.Context(), "certs", false)
		require.NoError(t, err)
		root, err := s.List(t.Context(), "", false)
		require.NoError(t, err)

		assert.Equal(t, []string{"certs/a.com", "certs/b.com", "certs/readme"}, entries)
		assert.Equal(t, []string{"certs", "other"}, root)
	})

	t.Run("lists every entry under a directory recursively", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)
		seed(t, s, "certs/a.com/a.crt", "certs/a.com/a.key", "certs/readme", "other/x")

		entries, err := s.List(t.Context(), "certs", true)

		require.NoError(t, err)
		assert.Equal(t, []string{"certs/a.com", "certs/a.com/a.crt", "certs/a.com/a.key", "certs/readme"}, entries)
	})

	t.Run("reports an empty directory as not existing", func(t *testing.T) {
		t.Parallel()
		s, _ := newStorage(t)

		_, err := s.List(t.Context(), "certs", false)

		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("describes keys and directories", func(t *testing.T) {
		t.Parallel()
		s, clk := newStorage(t)
		seed(t, s, "certs/a.com/a.crt")

		file, err := s.Stat(t.Context(), "certs/a.com/a.crt")
		require.NoError(t, err)
		dir, err := s.Stat(t.Context(), "certs/a.com")
		require.NoError(t, err)
		_, missing := s.Stat(t.Context(), "certs/b.com")

		assert.Equal(t, "certs/a.com/a.crt", file.Key)
		assert.Equal(t, int64(len("certs/a.com/a.crt")), file.Size)
		assert.True(t, file.IsTerminal)
		assert.True(t, clk.Now().Equal(file.Modified))
		assert.Equal(t, "certs/a.com", dir.Key)
		assert.False(t, dir.IsTerminal)
		require.ErrorIs(t, missing, fs.ErrNotExist)
	})

	t.Run("returns query errors once the pool is closed", func(t *testing.T) {
		t.Parallel()
		db := openDB(t)
		s := storage.New(db, clock.System())
		db.Close()

		_, loadErr := s.Load(t.Context(), "k")
		_, listErr := s.List(t.Context(), "k", false)
		_, statErr := s.Stat(t.Context(), "k")

		require.Error(t, s.Store(t.Context(), "k", nil))
		require.Error(t, s.Delete(t.Context(), "k"))
		require.Error(t, loadErr)
		require.NotErrorIs(t, loadErr, fs.ErrNotExist)
		require.Error(t, listErr)
		require.Error(t, statErr)
		assert.False(t, s.Exists(t.Context(), "k"))
	})
}
