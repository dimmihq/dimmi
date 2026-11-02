package foundation

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/dimmihq/dimmi/framework/log"
)

func TestZapLogger(t *testing.T) {
	t.Parallel()

	t.Run("writes library lines through the application's logger", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: &out})
		require.NoError(t, err)

		library := zapLogger(logger).Named("acme").With(zap.String("issuer", "test"))
		library.Debug("hidden")
		library.Info("obtaining certificate", zap.Int("attempt", 2))
		library.Warn("slow authority")
		library.Error("order failed", zap.Error(errors.New("refused")))
		require.NoError(t, library.Sync())

		lines := out.String()
		assert.NotContains(t, lines, "hidden")
		assert.Contains(t, lines, `"msg":"obtaining certificate"`)
		assert.Contains(t, lines, `"issuer":"test"`)
		assert.Contains(t, lines, `"attempt":2`)
		assert.Contains(t, lines, `"logger":"acme"`)
		assert.Contains(t, lines, `"level":"WARN"`)
		assert.Contains(t, lines, `"level":"ERROR"`)
	})
}

func TestMergedFS(t *testing.T) {
	t.Parallel()

	merged := mergedFS{
		fstest.MapFS{"2_b.sql": {Data: []byte("b")}},
		fstest.MapFS{"1_a.sql": {Data: []byte("a")}},
	}

	t.Run("shows several directories as one", func(t *testing.T) {
		t.Parallel()

		names, err := fs.Glob(merged, "*.sql")
		require.NoError(t, err)
		entries, err := fs.ReadDir(merged, ".")
		require.NoError(t, err)
		contents, err := fs.ReadFile(merged, "1_a.sql")
		require.NoError(t, err)

		assert.Equal(t, []string{"1_a.sql", "2_b.sql"}, names)
		assert.Equal(t, "1_a.sql", entries[0].Name())
		assert.Equal(t, "a", string(contents))
	})

	t.Run("reports what none of them holds", func(t *testing.T) {
		t.Parallel()

		_, err := merged.Open("3_c.sql")
		require.ErrorIs(t, err, fs.ErrNotExist)
		_, err = merged.Glob("[")
		require.Error(t, err)
		_, err = merged.ReadDir("missing")
		require.Error(t, err)
	})
}

func TestDescribeDatabase(t *testing.T) {
	t.Parallel()

	t.Run("names host, port and database without credentials", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "db.internal:5432/app", describeDatabase("postgres://user:secret@db.internal:5432/app"))
		assert.Equal(t, "(unreadable url)", describeDatabase("postgres://%zz"))
	})
}

func TestParseSize(t *testing.T) {
	t.Parallel()

	t.Run("reads sizes with binary suffixes and refuses the rest", func(t *testing.T) {
		t.Parallel()

		sizes := map[string]uint64{"1024": 1024, "2KiB": 2 << 10, "512MiB": 512 << 20, "1GiB": 1 << 30}
		for value, want := range sizes {
			got, err := parseSize(value)
			require.NoError(t, err)
			assert.Equal(t, want, got, value)
		}

		for _, value := range []string{"0", "-1", "lots", "1TiB"} {
			_, err := parseSize(value)
			require.Error(t, err, value)
		}
	})
}
