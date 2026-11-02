package health_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/health"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
openDatabase returns a pool on a fresh test database.
*/
func openDatabase(t *testing.T) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL:      testkit.NewDatabase(t).URL,
		MaxConns: 2,
		PoolMode: database.PoolModeSession,
	})
	require.NoError(t, err)

	return db
}

func TestDatabase(t *testing.T) {
	t.Parallel()

	t.Run("passes while postgres answers", func(t *testing.T) {
		t.Parallel()
		db := openDatabase(t)
		t.Cleanup(db.Close)
		check := health.Database(db)

		result := check.Run(t.Context())

		assert.Equal(t, "database", check.Name())
		assert.Equal(t, health.Result{Status: health.Pass, Detail: "reachable"}, result)
	})

	t.Run("fails with a fix once the pool is closed", func(t *testing.T) {
		t.Parallel()
		db := openDatabase(t)
		db.Close()

		result := health.Database(db).Run(t.Context())

		assert.Equal(t, health.Fail, result.Status)
		assert.Contains(t, result.Detail, "ping database")
		assert.NotEmpty(t, result.Fix)
	})
}

func TestValkey(t *testing.T) {
	t.Parallel()

	t.Run("passes while valkey answers", func(t *testing.T) {
		t.Parallel()
		check := health.Valkey(testkit.NewValkey(t))

		result := check.Run(t.Context())

		assert.Equal(t, "valkey", check.Name())
		assert.Equal(t, health.Pass, result.Status)
	})

	t.Run("fails with a fix once the client is closed", func(t *testing.T) {
		t.Parallel()
		client, err := redis.Open(t.Context(), testkit.ValkeyConfig(t))
		require.NoError(t, err)
		client.Close()

		result := health.Valkey(client).Run(t.Context())

		assert.Equal(t, health.Fail, result.Status)
		assert.Contains(t, result.Detail, "ping valkey")
		assert.NotEmpty(t, result.Fix)
	})
}
