package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
)

type mailConfig struct {
	Default string                    `default:"system" koanf:"default"`
	Mailers map[string]config.Section `koanf:"mailers"`
}

type appConfig struct {
	Name    string        `default:"app"  koanf:"name"`
	Debug   bool          `koanf:"debug"`
	Port    int           `default:"8080" koanf:"port"`
	Timeout time.Duration `default:"5s"   koanf:"timeout"`
	Secret  string        `koanf:"secret" secret:"true"`
	Mail    mailConfig    `koanf:"mail"`
}

type logConfig struct {
	Log struct {
		File string `koanf:"file"`
	} `koanf:"log"`
}

type listConfig struct {
	Locales []string `default:"en" koanf:"locales"`
	HTTP    struct {
		Ports []int  `koanf:"ports"`
		Key   []byte `koanf:"key"`
	} `koanf:"http"`
	Hosts map[string][]string `koanf:"hosts"`
}

type checkedConfig struct {
	Port int `koanf:"port"`
}

func (c checkedConfig) Validate() error {
	if c.Port == 0 {
		return errors.New("port is required")
	}

	return nil
}

/*
storeFunc is a runtime store whose settings come from a function, so
each test states exactly what the store returns.
*/
type storeFunc func(ctx context.Context) (map[string]any, error)

func (f storeFunc) Settings(ctx context.Context) (map[string]any, error) {
	return f(ctx)
}

/*
writeFile writes the contents into a new file in a fresh temporary
directory and returns its path.
*/
func writeFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("starts from defaults", func(t *testing.T) {
		t.Parallel()

		cfg, err := config.Load[appConfig](t.Context(), config.LoadOptions{Environ: []string{}})

		require.NoError(t, err)
		want := appConfig{Name: "app", Port: 8080, Timeout: 5 * time.Second, Mail: mailConfig{Default: "system"}}
		assert.Equal(t, want, *cfg)
	})

	t.Run("layers files then environment then secret files then runtime settings", func(t *testing.T) {
		t.Parallel()
		base := writeFile(t, "base.yaml", "name: base\nport: 9000\nsecret: from-file\nmail:\n  default: support\n")
		local := writeFile(t, "local.yaml", "port: 9100\nmail:\n  mailers:\n    system:\n      driver: smtp\n")
		secret := writeFile(t, "secret", "  from-secret-file\n")
		store := storeFunc(func(context.Context) (map[string]any, error) {
			return map[string]any{"mail.default": "runtime", "timeout": "1m"}, nil
		})

		cfg, err := config.Load[appConfig](t.Context(), config.LoadOptions{
			Name:      "app",
			Files:     []string{base, local},
			EnvPrefix: "APP",
			Environ: []string{
				"APP_DEBUG=true",
				"APP_SECRET=from-env",
				"APP_SECRET_FILE=" + secret,
				"APP_MAIL__DEFAULT=env",
				"APP_MAIL__MAILERS__SYSTEM__HOST=mail.local",
				"OTHER_PORT=1",
			},
			Runtime: store,
		})

		require.NoError(t, err)
		assert.Equal(t, "base", cfg.Name)
		assert.True(t, cfg.Debug)
		assert.Equal(t, 9100, cfg.Port)
		assert.Equal(t, time.Minute, cfg.Timeout)
		assert.Equal(t, "from-secret-file", cfg.Secret)
		assert.Equal(t, "runtime", cfg.Mail.Default)
		assert.Equal(t, config.Section{"driver": "smtp", "host": "mail.local"}, cfg.Mail.Mailers["system"])
	})

	t.Run("treats a double underscore before file as a plain key", func(t *testing.T) {
		t.Parallel()

		cfg, err := config.Load[logConfig](t.Context(), config.LoadOptions{
			EnvPrefix: "APP_",
			Environ:   []string{"APP_LOG__FILE=/var/log/app.log"},
		})

		require.NoError(t, err)
		assert.Equal(t, "/var/log/app.log", cfg.Log.File)
	})

	t.Run("splits comma-separated environment lists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ports"), []byte("80, 443\n"), 0o600))

		cfg, err := config.Load[listConfig](t.Context(), config.LoadOptions{
			EnvPrefix: "APP",
			Environ: []string{
				"APP_LOCALES=en, fr,,de ",
				"APP_HTTP__PORTS_FILE=" + filepath.Join(dir, "ports"),
				"APP_HTTP__KEY=a,b",
				"APP_HOSTS__WEB=a,b",
			},
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"en", "fr", "de"}, cfg.Locales)
		assert.Equal(t, []int{80, 443}, cfg.HTTP.Ports)
		assert.Equal(t, []byte("a,b"), cfg.HTTP.Key)
		assert.Equal(t, []string{"a,b"}, cfg.Hosts["web"])
	})

	t.Run("keeps yaml lists as written", func(t *testing.T) {
		t.Parallel()
		path := writeFile(t, "app.yaml", "locales:\n  - en,fr\n  - de\n")

		cfg, err := config.Load[listConfig](t.Context(), config.LoadOptions{Files: []string{path}})

		require.NoError(t, err)
		assert.Equal(t, []string{"en,fr", "de"}, cfg.Locales)
	})

	t.Run("reports a target defaults cannot fill", func(t *testing.T) {
		t.Parallel()

		_, err := config.Load[int](t.Context(), config.LoadOptions{Name: "app", Environ: []string{}})

		require.ErrorContains(t, err, "apply app defaults")
	})

	t.Run("reports every unreadable secret file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		_, err := config.Load[appConfig](t.Context(), config.LoadOptions{
			Name:      "app",
			EnvPrefix: "APP",
			Environ: []string{
				"APP_SECRET_FILE=" + filepath.Join(dir, "missing"),
				"APP_MAIL__DEFAULT_FILE=" + filepath.Join(dir, "absent"),
			},
		})

		require.ErrorIs(t, err, os.ErrNotExist)
		require.ErrorContains(t, err, "load app config")
		require.ErrorContains(t, err, "APP_SECRET_FILE")
		require.ErrorContains(t, err, "APP_MAIL__DEFAULT_FILE")
	})

	t.Run("skips a missing default file but not a listed one", func(t *testing.T) {
		t.Parallel()
		missing := filepath.Join(t.TempDir(), "missing.yaml")

		_, err := config.Load[appConfig](t.Context(), config.LoadOptions{Name: "absent-default-file"})
		require.NoError(t, err)

		_, err = config.Load[appConfig](t.Context(), config.LoadOptions{Name: "app", Files: []string{missing}})
		require.ErrorIs(t, err, os.ErrNotExist)
		require.ErrorContains(t, err, "read "+missing)
	})

	t.Run("reports a file that is not yaml", func(t *testing.T) {
		t.Parallel()
		broken := writeFile(t, "broken.yaml", "port: [unclosed\n")

		_, err := config.Load[appConfig](t.Context(), config.LoadOptions{Name: "app", Files: []string{broken}})

		require.ErrorContains(t, err, "read "+broken)
	})

	t.Run("reports a value of the wrong type", func(t *testing.T) {
		t.Parallel()

		_, err := config.Load[appConfig](t.Context(), config.LoadOptions{
			Name:      "app",
			EnvPrefix: "APP",
			Environ:   []string{"APP_PORT=eighty"},
		})

		require.ErrorContains(t, err, "decode app config")
	})

	t.Run("reports a runtime store that fails", func(t *testing.T) {
		t.Parallel()
		store := storeFunc(func(context.Context) (map[string]any, error) {
			return nil, errors.New("settings table missing")
		})

		_, err := config.Load[appConfig](t.Context(), config.LoadOptions{Name: "app", Runtime: store})

		require.ErrorContains(t, err, "read runtime settings: settings table missing")
	})

	t.Run("passes the context to the runtime store", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		store := storeFunc(func(ctx context.Context) (map[string]any, error) {
			return nil, ctx.Err()
		})

		_, err := config.Load[appConfig](ctx, config.LoadOptions{Name: "app", Runtime: store})

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("validates the loaded struct", func(t *testing.T) {
		t.Parallel()

		_, err := config.Load[checkedConfig](t.Context(), config.LoadOptions{Name: "app", Environ: []string{}})
		require.ErrorContains(t, err, "validate app config: port is required")

		cfg, err := config.Load[checkedConfig](t.Context(), config.LoadOptions{
			Name:      "app",
			EnvPrefix: "APP",
			Environ:   []string{"APP_PORT=80"},
		})
		require.NoError(t, err)
		assert.Equal(t, 80, cfg.Port)
	})

	t.Run("reads the process environment by default", func(t *testing.T) {
		t.Parallel()

		cfg, err := config.Load[appConfig](t.Context(), config.LoadOptions{EnvPrefix: "NO_SUCH_PREFIX_FOR_TESTS"})

		require.NoError(t, err)
		assert.Equal(t, 8080, cfg.Port)
	})
}

func TestLoadInto(t *testing.T) {
	t.Parallel()

	t.Run("fills the struct the caller holds", func(t *testing.T) {
		t.Parallel()
		cfg := &appConfig{}

		err := config.LoadInto(t.Context(), config.LoadOptions{
			EnvPrefix: "APP",
			Environ:   []string{"APP_PORT=9000", "APP_MAIL__DEFAULT=log"},
		}, cfg)

		require.NoError(t, err)
		assert.Equal(t, 9000, cfg.Port)
		assert.Equal(t, "log", cfg.Mail.Default)
		assert.Equal(t, "app", cfg.Name)
	})

	t.Run("splits environment lists of the struct", func(t *testing.T) {
		t.Parallel()
		cfg := &listConfig{}

		err := config.LoadInto(t.Context(), config.LoadOptions{
			EnvPrefix: "APP",
			Environ:   []string{"APP_LOCALES=en,fr"},
		}, cfg)

		require.NoError(t, err)
		assert.Equal(t, []string{"en", "fr"}, cfg.Locales)
	})

	t.Run("refuses a target that is not a struct pointer", func(t *testing.T) {
		t.Parallel()

		err := config.LoadInto(t.Context(), config.LoadOptions{Name: "app", Environ: []string{}}, 3)

		require.ErrorContains(t, err, "apply app defaults")
	})

	t.Run("checks the values of the struct", func(t *testing.T) {
		t.Parallel()

		err := config.LoadInto(t.Context(), config.LoadOptions{Name: "app", Environ: []string{}}, &checkedConfig{})

		require.ErrorContains(t, err, "port is required")
	})
}
