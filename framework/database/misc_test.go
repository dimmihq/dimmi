package database_test

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestSessionURL(t *testing.T) {
	t.Parallel()

	t.Run("prefers the direct url", func(t *testing.T) {
		t.Parallel()
		cfg := database.Config{URL: "pooler", DirectURL: "direct", PoolMode: database.PoolModeTransaction}

		url, err := cfg.SessionURL()

		require.NoError(t, err)
		assert.Equal(t, "direct", url)
	})

	t.Run("falls back to the url without a transaction pooler", func(t *testing.T) {
		t.Parallel()
		cfg := database.Config{URL: "primary", PoolMode: database.PoolModeSession}

		url, err := cfg.SessionURL()

		require.NoError(t, err)
		assert.Equal(t, "primary", url)
	})

	t.Run("refuses a transaction pooler without a direct url", func(t *testing.T) {
		t.Parallel()
		cfg := database.Config{URL: "pooler", PoolMode: database.PoolModeTransaction}

		_, err := cfg.SessionURL()

		require.ErrorIs(t, err, database.ErrNoSessionURL)
	})
}

func TestNotify(t *testing.T) {
	t.Parallel()

	t.Run("delivers the payload to listeners", func(t *testing.T) {
		t.Parallel()
		target := testkit.NewDatabase(t)
		db := openURL(t, target.URL, database.PoolModeSession)
		listener, err := pgx.Connect(t.Context(), target.URL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = listener.Close(context.WithoutCancel(t.Context())) })
		_, err = listener.Exec(t.Context(), "LISTEN changes")
		require.NoError(t, err)

		require.NoError(t, db.Notify(t.Context(), "changes", "item 1"))

		notification, err := listener.WaitForNotification(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "changes", notification.Channel)
		assert.Equal(t, "item 1", notification.Payload)
	})

	t.Run("refuses to notify inside a transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, _ *database.Tx) error {
			return db.Notify(ctx, "changes", "too early")
		})

		require.ErrorIs(t, err, database.ErrNotifyInTransaction)
	})

	t.Run("reports a failed notify", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Notify(t.Context(), "", "x")

		require.ErrorContains(t, err, "notify")
	})
}

func TestCopyFrom(t *testing.T) {
	t.Parallel()

	t.Run("loads rows in the transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		mustExec(t, db, "CREATE SCHEMA archive; CREATE TABLE archive.items (id int, name text)")

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			n, err := tx.CopyFrom(ctx, "items", []string{"id", "name"}, [][]any{{1, "a"}, {2, "b"}})
			require.NoError(t, err)
			assert.EqualValues(t, 2, n)

			n, err = tx.CopyFrom(ctx, "archive.items", []string{"id", "name"}, [][]any{{3, "c"}})
			require.NoError(t, err)
			assert.EqualValues(t, 1, n)

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 2, countItems(t.Context(), t, db))
	})

	t.Run("reports a failed copy", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.CopyFrom(ctx, "missing", []string{"id"}, [][]any{{1}})

			return err
		})

		require.ErrorContains(t, err, "copy into missing")
	})

	t.Run("is refused on tables under row-level security", func(t *testing.T) {
		t.Parallel()
		db := openTenantNotes(t)

		err := db.Transaction(tenancy.With(t.Context(), "a"), func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.CopyFrom(ctx, "notes", []string{"tenant", "body"}, [][]any{{"a", "copied"}})

			return err
		})

		require.ErrorContains(t, err, "row-level security")
	})
}

func TestVector(t *testing.T) {
	t.Parallel()

	t.Run("round-trips through a vector column", func(t *testing.T) {
		t.Parallel()
		target := testkit.NewDatabase(t)
		admin, err := pgx.Connect(t.Context(), target.AdminURL)
		require.NoError(t, err)
		_, err = admin.Exec(t.Context(), "CREATE EXTENSION IF NOT EXISTS vector")
		require.NoError(t, err)
		require.NoError(t, admin.Close(t.Context()))

		for _, mode := range []database.PoolMode{database.PoolModeSession, database.PoolModeTransaction} {
			db := openURL(t, target.URL, mode)
			mustExec(t, db, "CREATE TABLE IF NOT EXISTS embeddings (id int, embedding vector(3))")
			mustExec(t, db, "TRUNCATE embeddings")

			stored := database.NewVector([]float32{1, 2, 3})
			_, err = db.Exec(t.Context(), "INSERT INTO embeddings VALUES (1, $1)", stored)
			require.NoError(t, err)

			var got database.Vector
			err = db.QueryRow(t.Context(), "SELECT embedding FROM embeddings ORDER BY embedding <-> $1 LIMIT 1",
				database.NewVector([]float32{1, 2, 4})).Scan(&got)
			require.NoError(t, err)
			assert.Equal(t, []float32{1, 2, 3}, got.Slice())
		}
	})
}

/*
recordingTracer keeps the SQL of every query it observes.
*/
type recordingTracer struct {
	mu  sync.Mutex
	sql []string
}

func (r *recordingTracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sql = append(r.sql, data.SQL)

	return ctx
}

func (*recordingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *recordingTracer) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.sql)
}

func TestTracer(t *testing.T) {
	t.Parallel()

	t.Run("observes every query on the pool", func(t *testing.T) {
		t.Parallel()
		tracer := &recordingTracer{}
		db, err := database.Open(t.Context(), database.Config{
			URL:      testkit.NewDatabase(t).URL,
			MaxConns: 2,
			PoolMode: database.PoolModeSession,
			Tracer:   tracer,
		})
		require.NoError(t, err)
		t.Cleanup(db.Close)

		var one int
		require.NoError(t, db.QueryRow(t.Context(), "SELECT 1").Scan(&one))

		assert.Contains(t, tracer.seen(), "SELECT 1")
	})
}
