package foundation_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/telemetry/otlp"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("completes every driver service with defaults that work on one machine", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()

		cfg, err := foundation.LoadConfig(t.Context(), options([]string{
			"APP_DATABASE__URL=postgres://localhost/app", "APP_REDIS__URL=redis://localhost",
			"APP_APP__DATA_DIR=" + dataDir,
		}))
		require.NoError(t, err)

		assert.Equal(t, foundation.EnvProduction, cfg.App.Env)
		all := foundation.Roles{foundation.RoleWeb, foundation.RoleWorker, foundation.RoleScheduler}
		assert.Equal(t, all, cfg.App.Roles)
		assert.Equal(t, "app", cfg.Telemetry.ServiceName)
		assert.Equal(t, "production", cfg.Telemetry.Environment)
		assert.Equal(t, "app:", cfg.Redis.KeyPrefix)
		assert.Equal(t, "argon2id", cfg.Hashing.Default)
		assert.Equal(t, filepath.Join(dataDir, "app"), cfg.Filesystems.Disks["local"]["root"])
		assert.Equal(t, "log", cfg.Mail.Mailers["system"]["driver"])
		assert.Equal(t, "none", cfg.Malware.Default)
		assert.Equal(t, "local", cfg.Encryption.KEK)
		assert.Equal(t, "none", cfg.Languages.Default)
		assert.Equal(t, "none", cfg.CustomDomains.Default)
		assert.False(t, cfg.Mail.AllowFakes)
		assert.True(t, cfg.Migrations.Auto)
	})

	t.Run("lists every problem at once with the configuration masked", func(t *testing.T) {
		t.Parallel()

		_, err := foundation.LoadConfig(t.Context(), options([]string{
			"APP_APP__ENV=staging", "APP_APP__ROLES=web,cron", "APP_APP__URL=ftp://example.com",
			"APP_APP__KEY=base64:secret", "APP_RUNTIME__MEMORY_LIMIT=lots", "APP_DATABASE__POOL_MODE=odd",
			"APP_HTTP__SHUTDOWN_TIMEOUT=0s", "APP_APP__DATA_DIR=",
		}))

		var configErr *foundation.ConfigError
		require.ErrorAs(t, err, &configErr)
		require.ErrorIs(t, err, foundation.ErrInvalidConfig)
		require.ErrorIs(t, err, foundation.ErrInvalidEnvironment)
		require.ErrorIs(t, err, foundation.ErrInvalidRole)
		for _, key := range []string{
			"app.url", "database.url", "redis.url", "runtime.memory_limit", "database.pool_mode",
			"http.shutdown_timeout", "app.data_dir",
		} {
			assert.ErrorContains(t, err, key)
		}

		app, ok := configErr.Redacted["app"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "[redacted]", app["key"])
	})

	t.Run("reports configuration it cannot read", func(t *testing.T) {
		t.Parallel()
		opts := options([]string{"APP_HTTP__IDLE_TIMEOUT=forever"})

		_, err := foundation.LoadConfig(t.Context(), opts)

		var configErr *foundation.ConfigError
		require.ErrorAs(t, err, &configErr)
		assert.ErrorContains(t, configErr, "decode app config")
	})

	t.Run("reads the files the application names", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "app.yaml")
		yaml := "database:\n  url: postgres://localhost/app\nredis:\n  url: redis://localhost\n" +
			"runtime:\n  memory_limit: 512MiB\nhashing:\n  default: bcrypt\n  hashers:\n    bcrypt: {driver: bcrypt}\n"
		require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
		opts := options(nil)
		opts.ConfigFiles = []string{path}
		opts.Environ = []string{}

		cfg, err := foundation.LoadConfig(t.Context(), opts)
		require.NoError(t, err)

		assert.Equal(t, "512MiB", cfg.Runtime.MemoryLimit)
		assert.Equal(t, "bcrypt", cfg.Hashing.Default)
		assert.NotContains(t, cfg.Hashing.Hashers, "argon2id")
	})
}

func TestBootFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		env  []string
		want string
	}{
		"refuses an app key that is not 32 bytes": {[]string{"APP_APP__KEY=base64:c2hvcnQ="}, "app.key"},
		"refuses to run without any app key":      {[]string{"APP_APP__KEY="}, "no app key"},
		"stops when the database cannot be reached": {
			[]string{"APP_DATABASE__URL=postgres://127.0.0.1:1/app"}, "open database",
		},
		"stops when valkey cannot be reached": {[]string{"APP_REDIS__URL=redis://127.0.0.1:1"}, "open valkey"},
		"stops on an unknown telemetry driver": {
			[]string{"APP_TELEMETRY__TRACING=carrier-pigeon"}, "start telemetry",
		},
		"stops on an unknown log driver": {[]string{"APP_LOG__DRIVER=xml"}, "start logger"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := foundation.New(t.Context(), options(environ(t, c.env...)))

			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), c.want)
		})
	}
}

func TestBootTelemetry(t *testing.T) {
	t.Parallel()

	t.Run("traces through an exporter the application registers", func(t *testing.T) {
		t.Parallel()
		collector := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		t.Cleanup(collector.Close)
		opts := options(environ(t, "APP_TELEMETRY__TRACING=otlp",
			"APP_TELEMETRY__OTLP_ENDPOINT="+collector.URL+"/v1/traces"))
		opts.Telemetry = []telemetry.Option{otlp.Tracing()}

		app := newApp(t, opts)

		_, span := app.Telemetry.TracerProvider().Tracer("test").Start(t.Context(), "boot")
		assert.True(t, span.SpanContext().IsValid())
		span.End()
	})
}

func TestBootMigrations(t *testing.T) {
	t.Parallel()

	t.Run("stops on a migration that fails", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t))
		opts.Migrations = fstest.MapFS{"20261102170000_broken.sql": {Data: []byte(
			"-- migrate: phase=expand\n\n-- +goose Up\nCREATE TABLE;\n",
		)}}

		_, err := foundation.New(t.Context(), opts)

		require.ErrorContains(t, err, "migrate on boot")
	})

	t.Run("leaves the schema alone when automatic migration is off", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(freshEnviron(t, "APP_MIGRATIONS__AUTO=false")))

		pending, err := app.Migrator.Plan(t.Context())
		require.NoError(t, err)

		assert.NotEmpty(t, pending)
	})

	t.Run("refuses to migrate behind a transaction pooler without a direct url", func(t *testing.T) {
		t.Parallel()

		_, err := foundation.New(t.Context(), options(environ(t, "APP_DATABASE__POOL_MODE=transaction")))

		require.ErrorContains(t, err, "migrations")
	})
}

func TestAppKey(t *testing.T) {
	t.Parallel()

	t.Run("generates a key file once and reads it on every later boot", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		env := environ(t, "APP_APP__KEY=", "APP_APP__KEY_AUTO_GENERATE=true", "APP_APP__DATA_DIR="+dataDir,
			"APP_MIGRATIONS__AUTO=false")

		first := newApp(t, options(env))
		key, err := os.ReadFile(filepath.Clean(filepath.Join(dataDir, "app.key")))
		require.NoError(t, err)
		second := newApp(t, options(env))

		sealed, err := first.Encrypter.EncryptString("secret")
		require.NoError(t, err)
		opened, err := second.Encrypter.DecryptString(sealed)
		require.NoError(t, err)
		assert.Equal(t, "secret", opened)
		assert.True(t, strings.HasPrefix(string(key), "base64:"))
		info, err := os.Stat(filepath.Join(dataDir, "app.key"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("refuses a key file it cannot read as a key", func(t *testing.T) {
		t.Parallel()
		dataDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, "app.key"), []byte("garbage\n"), 0o600))

		_, err := foundation.New(t.Context(), options(environ(t, "APP_APP__KEY=", "APP_APP__DATA_DIR="+dataDir)))

		require.ErrorContains(t, err, "app.key")
	})

	t.Run("reports a data directory it cannot write the key to", func(t *testing.T) {
		t.Parallel()
		blocked := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(blocked, nil, 0o600))

		_, err := foundation.New(t.Context(), options(environ(t,
			"APP_APP__KEY=", "APP_APP__KEY_AUTO_GENERATE=true", "APP_APP__DATA_DIR="+blocked)))

		require.Error(t, err)
	})
}
