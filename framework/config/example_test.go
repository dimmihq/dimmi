package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
)

/*
exampleLimits is a section a second struct shares with the first.
*/
type exampleLimits struct {
	Burst *int     `koanf:"burst"`
	Hosts []string `default:"a.test, b.test" koanf:"hosts"`
}

type exampleClient struct {
	URL   string        `koanf:"url"`
	Every time.Duration `default:"1h" koanf:"every"`
}

type exampleFirst struct {
	App      appConfig                `koanf:"app"`
	Limits   exampleLimits            `koanf:"limits"`
	Clients  map[string]exampleClient `koanf:"clients"`
	Labels   map[string]string        `koanf:"labels"`
	Optional *exampleClient           `koanf:"optional"`
	Hidden   string                   `koanf:"-"`
}

type exampleSecond struct {
	Limits struct {
		Window time.Duration `default:"90s" koanf:"window"`
	} `koanf:"limits"`
}

func writeExample(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "example.yaml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

func TestExample(t *testing.T) {
	t.Parallel()

	t.Run("lists every setting at its default beside its variable", func(t *testing.T) {
		t.Parallel()

		out, err := config.Example(config.ExampleOptions{Header: "The settings.\n\nAll of them.", EnvPrefix: "APP"},
			&exampleFirst{}, &exampleSecond{})
		require.NoError(t, err)

		text := string(out)
		assert.Contains(t, text, "# The settings.\n#\n# All of them.\n")
		assert.Contains(t, text, "app:\n  name: app  # APP_APP__NAME\n  debug: false  # APP_APP__DEBUG\n")
		assert.Contains(t, text, "  timeout: 5s  # APP_APP__TIMEOUT\n")
		assert.Contains(t, text, `  secret: ""  # APP_APP__SECRET (secret; or APP_APP__SECRET_FILE naming a file)`)
		assert.Contains(t, text, "  mail:\n    default: system  # APP_APP__MAIL__DEFAULT\n")
		assert.Contains(t, text, "    mailers: {}  # APP_APP__MAIL__MAILERS__<NAME>__<OPTION>\n")
		assert.Contains(t, text, "  burst: null  # APP_LIMITS__BURST\n  hosts: [a.test, b.test]  # APP_LIMITS__HOSTS\n")
		assert.Contains(t, text, "  window: 1m30s  # APP_LIMITS__WINDOW\n")
		assert.Contains(t, text, "clients: {}  # APP_CLIENTS__<NAME>__<OPTION>\n")
		assert.Contains(t, text, "labels: {}  # APP_LABELS__<NAME>\n")
		assert.Contains(t, text, "optional: null  # APP_OPTIONAL\n")
		assert.NotContains(t, text, "hidden")
	})

	t.Run("keeps the values the example sets and loads back to them", func(t *testing.T) {
		t.Parallel()

		path := writeExample(t, `
app:
  name: "shop #1"
  port: 9000
  mail:
    mailers:
      system:
        driver: smtp
        port: 25
limits:
  burst: 5
  hosts: [c.test]
clients:
  billing:
    url: https://billing.test
labels:
  tier: gold
optional:
  url: https://optional.test
`)

		out, err := config.Example(config.ExampleOptions{EnvPrefix: "APP_", Path: path}, &exampleFirst{})
		require.NoError(t, err)

		text := string(out)
		assert.Contains(t, text, `  name: "shop #1"  # APP_APP__NAME`)
		assert.Contains(t, text, "      system:\n        driver: smtp  # APP_APP__MAIL__MAILERS__SYSTEM__DRIVER\n")
		assert.Contains(t, text, "  burst: 5  # APP_LIMITS__BURST\n  hosts: [c.test]  # APP_LIMITS__HOSTS\n")
		assert.Contains(t, text, "  billing:\n    url: https://billing.test  # APP_CLIENTS__BILLING__URL\n")
		assert.Contains(t, text, "    every: 1h  # APP_CLIENTS__BILLING__EVERY\n")
		assert.Contains(t, text, "  tier: gold  # APP_LABELS__TIER\n")
		assert.Contains(t, text, "optional:\n  url: https://optional.test  # APP_OPTIONAL__URL\n")

		again := writeExample(t, text)
		cfg, err := config.Load[exampleFirst](t.Context(), config.LoadOptions{Name: "app", Files: []string{again}})
		require.NoError(t, err)
		assert.Equal(t, "shop #1", cfg.App.Name)
		assert.Equal(t, 9000, cfg.App.Port)
		assert.Equal(t, 5, *cfg.Limits.Burst)
		assert.Equal(t, time.Hour, cfg.Clients["billing"].Every)

		twice, err := config.Example(config.ExampleOptions{EnvPrefix: "APP", Path: again}, &exampleFirst{})
		require.NoError(t, err)
		assert.Equal(t, text, string(twice))
	})

	t.Run("refuses a key no struct reads", func(t *testing.T) {
		t.Parallel()

		path := writeExample(t, "app:\n  title: none\nextra: 1\n")

		_, err := config.Example(config.ExampleOptions{EnvPrefix: "APP", Path: path}, &exampleFirst{})
		require.ErrorIs(t, err, config.ErrUnknownExampleKey)
		assert.ErrorContains(t, err, "app.title, extra")
	})

	t.Run("refuses an example that is not YAML and a target that is not a struct", func(t *testing.T) {
		t.Parallel()

		_, err := config.Example(config.ExampleOptions{Path: writeExample(t, "app: [")}, &exampleFirst{})
		require.ErrorContains(t, err, "parse the current example")

		_, err = config.Example(config.ExampleOptions{Path: t.TempDir()}, &exampleFirst{})
		require.ErrorContains(t, err, "read the current example")

		_, err = config.Example(config.ExampleOptions{}, "not a struct")
		require.ErrorContains(t, err, "apply defaults")
	})

	t.Run("says what each struct's comment says", func(t *testing.T) {
		t.Parallel()

		out, err := config.Example(config.ExampleOptions{
			EnvPrefix: "APP",
			Module:    "github.com/dimmihq/dimmi",
			Root:      "../..",
		}, &struct {
			Limits exampleLimits `koanf:"limits"`
		}{})
		require.NoError(t, err)
		assert.Contains(t, string(out),
			"# exampleLimits is a section a second struct shares with the first.\nlimits:\n")
	})
}
