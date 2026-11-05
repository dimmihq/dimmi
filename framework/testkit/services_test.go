package testkit_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestEnv(t *testing.T) {
	t.Parallel()

	t.Run("returns a variable that is set", func(t *testing.T) {
		t.Parallel()

		assert.NotEmpty(t, testkit.Env(t, "DATABASE_URL"))
	})

	t.Run("fails loudly on a variable that is not set", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) { testkit.Env(tb, "TESTKIT_NEVER_SET") })

		assert.Contains(t, rec.Failures()[0], "TESTKIT_NEVER_SET is not set")
	})

	t.Run("a switch is on only when set to 1", func(t *testing.T) {
		t.Parallel()

		assert.False(t, testkit.Switch("TESTKIT_NEVER_SET"))
		assert.False(t, testkit.Switch("DATABASE_URL"))
	})

	t.Run("names things uniquely", func(t *testing.T) {
		t.Parallel()

		name := testkit.UniqueName("t_")

		assert.Regexp(t, regexp.MustCompile(`^t_[a-z0-9]{10}$`), name)
		assert.NotEqual(t, name, testkit.UniqueName("t_"))
	})
}

func TestServices(t *testing.T) {
	t.Parallel()

	t.Run("gives each test its own database", func(t *testing.T) {
		t.Parallel()

		db := testkit.NewDatabase(t)

		require.NoError(t, exec(t.Context(), db.URL, "CREATE TABLE items (id int)"))
		require.NoError(t, exec(t.Context(), db.AdminURL, "SELECT count(*) FROM items"))
	})

	t.Run("gives each test its own valkey prefix", func(t *testing.T) {
		t.Parallel()

		client := testkit.NewValkey(t)

		assert.NotEqual(t, testkit.ValkeyConfig(t).KeyPrefix, testkit.ValkeyConfig(t).KeyPrefix)
		require.NotNil(t, client)
	})

	t.Run("gives each test its own bucket", func(t *testing.T) {
		t.Parallel()

		assert.NotEmpty(t, testkit.NewBucket(t))
	})
}
