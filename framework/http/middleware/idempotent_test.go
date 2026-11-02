package middleware_test

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func newStore(t *testing.T, table string) *idempotency.DatabaseStore {
	t.Helper()

	store, _ := openStore(t, table)

	return store
}

func openStore(t *testing.T, table string) (*idempotency.DatabaseStore, *database.DB) {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL:           testkit.NewDatabase(t).URL,
		MaxConns:      4,
		PoolMode:      database.PoolModeSession,
		TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	_, err = db.Exec(t.Context(), `CREATE TABLE idempotency_keys (
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
	)`)
	require.NoError(t, err)

	return idempotency.NewDatabaseStore(db, idempotency.Config{Table: table}, clock.System()), db
}

/*
counter answers with the status in the status query parameter, or 201,
and counts how often it ran.
*/
func counter(runs *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runs.Add(1)
		w.Header().Set("Location", "/items/1")
		status := map[string]int{"": http.StatusCreated, "500": http.StatusInternalServerError}
		w.WriteHeader(status[r.URL.Query().Get("status")])
		_, _ = w.Write([]byte(`{"run":` + strconv.Itoa(int(runs.Load())) + `}`))
	})
}

func keyed(t *testing.T, target, key, body string) *http.Request {
	t.Helper()

	r := request(t, http.MethodPost, target, body)
	r.Header.Set("Idempotency-Key", key)

	return r
}

func TestIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("replays the first response to a retry", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		h := middleware.Idempotent(newStore(t, ""), "", nil)(counter(&runs))

		first := serve(h, keyed(t, "/items", "k1", `{"a":1}`))
		retry := serve(h, keyed(t, "/items", "k1", `{"a":1}`))

		assert.Equal(t, int32(1), runs.Load())
		assert.Equal(t, http.StatusCreated, retry.Code)
		assert.Equal(t, first.Body.String(), retry.Body.String())
		assert.Equal(t, "/items/1", retry.Header().Get("Location"))
		assert.Equal(t, "true", retry.Header().Get(middleware.ReplayedHeader))
		assert.Empty(t, first.Header().Get(middleware.ReplayedHeader))
	})

	t.Run("refuses the key on a different request", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		h := middleware.Idempotent(newStore(t, ""), "", nil)(counter(&runs))
		serve(h, keyed(t, "/items", "k1", `{"a":1}`))

		rec := serve(h, keyed(t, "/items", "k1", `{"a":2}`))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Equal(t, "idempotency_key_reused", code(t, rec.Body))
	})

	t.Run("refuses a retry while the first is running", func(t *testing.T) {
		t.Parallel()
		store := newStore(t, "")
		var nested string
		var h http.Handler
		h = middleware.Idempotent(store, "", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if nested == "" {
				retry := httptest.NewRequestWithContext(r.Context(), http.MethodPost, "/items", strings.NewReader(""))
				retry.Header.Set("Idempotency-Key", "k1")
				rec := serve(h, retry)
				nested = code(t, rec.Body)
			}
			w.WriteHeader(http.StatusCreated)
		}))

		serve(h, keyed(t, "/items", "k1", ""))
		assert.Equal(t, "idempotency_key_in_use", nested)
	})

	t.Run("runs again after a server error", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		h := middleware.Idempotent(newStore(t, ""), "", nil)(counter(&runs))

		assert.Equal(t, http.StatusInternalServerError, serve(h, keyed(t, "/items?status=500", "k1", "")).Code)
		assert.Equal(t, http.StatusInternalServerError, serve(h, keyed(t, "/items?status=500", "k1", "")).Code)
		assert.Equal(t, int32(2), runs.Load())
	})

	t.Run("keeps keys apart per scope", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		scope := func(r *http.Request) string { return r.Header.Get("X-User") }
		h := middleware.Idempotent(newStore(t, ""), "", scope)(counter(&runs))

		for _, user := range []string{"a", "b"} {
			r := keyed(t, "/items", "k1", "")
			r.Header.Set("X-User", user)
			assert.Equal(t, http.StatusCreated, serve(h, r).Code)
		}
		assert.Equal(t, int32(2), runs.Load())
	})

	t.Run("ignores requests without a key and other methods", func(t *testing.T) {
		t.Parallel()
		var runs atomic.Int32
		h := middleware.Idempotent(newStore(t, ""), "", nil)(counter(&runs))
		patch := keyed(t, "/items", "k1", "")
		patch.Method = http.MethodPatch

		serve(h, request(t, http.MethodPost, "/items", ""))
		serve(h, request(t, http.MethodPost, "/items", ""))
		serve(h, patch)
		assert.Equal(t, int32(3), runs.Load())
	})

	t.Run("refuses keys that are too long", func(t *testing.T) {
		t.Parallel()
		h := middleware.Idempotent(newStore(t, ""), "", nil)(echo())
		rec := serve(h, keyed(t, "/items", strings.Repeat("k", 256), ""))

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Equal(t, "invalid_idempotency_key", code(t, rec.Body))
	})

	t.Run("refuses bodies it cannot read", func(t *testing.T) {
		t.Parallel()
		h := middleware.BodyLimit(2)(middleware.Idempotent(newStore(t, ""), "", nil)(echo()))
		r := keyed(t, "/items", "k1", "12345")
		r.ContentLength = -1

		assert.Equal(t, http.StatusRequestEntityTooLarge, serve(h, r).Code)
	})

	t.Run("answers 500 when the store fails", func(t *testing.T) {
		t.Parallel()
		h := middleware.Idempotent(newStore(t, "missing"), "", nil)(echo())

		assert.Equal(t, http.StatusInternalServerError, serve(h, keyed(t, "/items", "k1", "")).Code)
	})

	t.Run("still answers when the response cannot be stored", func(t *testing.T) {
		t.Parallel()
		store, db := openStore(t, "")
		h := middleware.Idempotent(store, "", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := db.Exec(r.Context(), "DROP TABLE idempotency_keys")
			assert.NoError(t, err)
			w.WriteHeader(http.StatusCreated)
		}))

		assert.Equal(t, http.StatusCreated, serve(h, keyed(t, "/items", "k1", "")).Code)
	})
}
