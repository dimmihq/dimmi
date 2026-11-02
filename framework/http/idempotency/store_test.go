package idempotency_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
schema creates the store's table under the given name.
*/
func schema(table string) string {
	return "CREATE TABLE " + table + ` (
		tenant_id   text        NOT NULL DEFAULT '',
		scope       text        NOT NULL,
		key         text        NOT NULL,
		fingerprint text        NOT NULL,
		status      integer,
		header      jsonb,
		body        bytea,
		created_at  timestamptz NOT NULL,
		expires_at  timestamptz NOT NULL,
		PRIMARY KEY (tenant_id, scope, key)
	)`
}

/*
openDatabase returns a pool on a fresh database holding the store's
table, under the default name and as audit.requests.
*/
func openDatabase(t *testing.T) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL:           testkit.NewDatabase(t).URL,
		MaxConns:      4,
		PoolMode:      database.PoolModeSession,
		TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	for _, sql := range []string{schema("idempotency_keys"), "CREATE SCHEMA audit", schema("audit.requests")} {
		_, err := db.Exec(t.Context(), sql)
		require.NoError(t, err)
	}

	return db
}

func newStore(t *testing.T, cfg idempotency.Config) (*idempotency.DatabaseStore, *clock.Fake) {
	t.Helper()

	clk := clock.NewFake(time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC))

	return idempotency.NewDatabaseStore(openDatabase(t), cfg, clk), clk
}

func TestDatabaseStore(t *testing.T) {
	t.Parallel()

	created := idempotency.Response{
		Status: http.StatusCreated,
		Header: http.Header{"Content-Type": {"application/json"}, "Set-Cookie": {"session=secret"}},
		Body:   []byte(`{"data":{}}`),
	}

	t.Run("runs the first request and replays its response", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{})
		ctx := t.Context()

		first, err := s.Reserve(ctx, "usr_1", "k1", "fp")
		require.NoError(t, err)
		require.Nil(t, first)
		require.NoError(t, s.Complete(ctx, "usr_1", "k1", created))

		replay, err := s.Reserve(ctx, "usr_1", "k1", "fp")
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, replay.Status)
		assert.Equal(t, created.Body, replay.Body)
		assert.Equal(t, "application/json", replay.Header.Get("Content-Type"))
		assert.Empty(t, replay.Header.Values("Set-Cookie"))
	})

	t.Run("refuses a second request while the first runs", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{})
		_, err := s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.NoError(t, err)

		_, err = s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.ErrorIs(t, err, idempotency.ErrInProgress)
	})

	t.Run("refuses a key reused for another request", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{})
		_, err := s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.NoError(t, err)

		_, err = s.Reserve(t.Context(), "usr_1", "k1", "other")
		require.ErrorIs(t, err, idempotency.ErrKeyReused)
	})

	t.Run("keeps keys apart per scope and tenant", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{})
		_, err := s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.NoError(t, err)

		other, err := s.Reserve(t.Context(), "usr_2", "k1", "fp")
		require.NoError(t, err)
		assert.Nil(t, other)

		tenant, err := s.Reserve(tenancy.With(t.Context(), "tnt_1"), "usr_1", "k1", "fp")
		require.NoError(t, err)
		assert.Nil(t, tenant)
	})

	t.Run("runs again after a release", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{})
		_, err := s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.NoError(t, err)
		require.NoError(t, s.Release(t.Context(), "usr_1", "k1"))

		again, err := s.Reserve(t.Context(), "usr_1", "k1", "other")
		require.NoError(t, err)
		assert.Nil(t, again)
	})

	t.Run("takes over abandoned reservations and expired keys", func(t *testing.T) {
		t.Parallel()
		s, clk := newStore(t, idempotency.Config{TTL: time.Hour, LockTimeout: time.Minute})
		_, err := s.Reserve(t.Context(), "usr_1", "busy", "fp")
		require.NoError(t, err)
		_, err = s.Reserve(t.Context(), "usr_1", "done", "fp")
		require.NoError(t, err)
		require.NoError(t, s.Complete(t.Context(), "usr_1", "done", created))

		clk.Travel(time.Minute)
		busy, err := s.Reserve(t.Context(), "usr_1", "busy", "fp")
		require.NoError(t, err)
		assert.Nil(t, busy)
		_, err = s.Reserve(t.Context(), "usr_1", "done", "other")
		require.ErrorIs(t, err, idempotency.ErrKeyReused)

		clk.Travel(time.Hour)
		done, err := s.Reserve(t.Context(), "usr_1", "done", "other")
		require.NoError(t, err)
		assert.Nil(t, done)
	})

	t.Run("prunes expired keys", func(t *testing.T) {
		t.Parallel()
		s, clk := newStore(t, idempotency.Config{Table: "audit.requests", TTL: time.Hour})
		_, err := s.Reserve(t.Context(), "usr_1", "old", "fp")
		require.NoError(t, err)
		clk.Travel(30 * time.Minute)
		_, err = s.Reserve(t.Context(), "usr_1", "new", "fp")
		require.NoError(t, err)

		clk.Travel(30 * time.Minute)
		deleted, err := s.Prune(t.Context())
		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)
	})

	t.Run("wraps database failures", func(t *testing.T) {
		t.Parallel()
		s, _ := newStore(t, idempotency.Config{Table: "missing"})

		_, err := s.Reserve(t.Context(), "usr_1", "k1", "fp")
		require.ErrorContains(t, err, "reserve idempotency key")
		require.ErrorContains(t, s.Complete(t.Context(), "usr_1", "k1", created), "complete idempotency key")
		require.ErrorContains(t, s.Release(t.Context(), "usr_1", "k1"), "release idempotency key")
		_, err = s.Prune(t.Context())
		require.ErrorContains(t, err, "prune idempotency keys")
	})
}
