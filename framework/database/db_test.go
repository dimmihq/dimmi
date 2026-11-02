package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	t.Run("connects to a primary", func(t *testing.T) {
		t.Parallel()
		db := openURL(t, testkit.NewDatabase(t).URL, database.PoolModeSession)

		require.NoError(t, db.Ping(t.Context()))
	})

	t.Run("runs parameterized queries behind a transaction pooler", func(t *testing.T) {
		t.Parallel()
		db := openURL(t, testkit.NewDatabase(t).URL, database.PoolModeTransaction)

		var sum int
		require.NoError(t, db.QueryRow(t.Context(), "SELECT $1::int + $2::int", 2, 3).Scan(&sum))
		assert.Equal(t, 5, sum)
	})

	t.Run("rejects an unknown pool mode", func(t *testing.T) {
		t.Parallel()

		_, err := database.Open(t.Context(), database.Config{URL: "postgres://localhost/x", PoolMode: "statement"})

		require.ErrorIs(t, err, database.ErrInvalidPoolMode)
	})

	t.Run("rejects a malformed url", func(t *testing.T) {
		t.Parallel()

		_, err := database.Open(t.Context(), database.Config{URL: "postgres://%zz", PoolMode: database.PoolModeSession})

		require.ErrorContains(t, err, "parse database url")
	})

	t.Run("fails when the server cannot be reached", func(t *testing.T) {
		t.Parallel()

		_, err := database.Open(t.Context(), database.Config{
			URL:      "postgres://nobody@127.0.0.1:1/none?connect_timeout=1",
			MaxConns: 1,
			PoolMode: database.PoolModeSession,
		})

		require.ErrorContains(t, err, "check primary")
	})
}

func TestDBQueries(t *testing.T) {
	t.Parallel()

	t.Run("runs statements outside a transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		tag, err := db.Exec(t.Context(), "INSERT INTO items VALUES (1, 'a'), (2, 'b')")
		require.NoError(t, err)
		assert.EqualValues(t, 2, tag.RowsAffected())

		rows, err := db.Query(t.Context(), "SELECT name FROM items ORDER BY id")
		require.NoError(t, err)
		defer rows.Close()

		var names []string
		for rows.Next() {
			var name string
			require.NoError(t, rows.Scan(&name))
			names = append(names, name)
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, []string{"a", "b"}, names)
	})

	t.Run("returns query errors", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		require.ErrorContains(t, queryError(t.Context(), db, "SELECT * FROM missing"), "missing")

		_, err := db.Exec(t.Context(), "DELETE FROM missing")
		require.ErrorContains(t, err, "missing")
	})
}
