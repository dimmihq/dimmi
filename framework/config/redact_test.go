package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/config"
)

type webhookConfig struct {
	URL    string `koanf:"url"`
	Secret string `koanf:"secret" secret:"true"`
}

type redactedConfig struct {
	Token    string                   `koanf:"token"    secret:"true"`
	Unset    string                   `koanf:"unset"    secret:"true"`
	Region   string                   `koanf:"region"`
	Hooks    []webhookConfig          `koanf:"hooks"`
	Clients  map[string]webhookConfig `koanf:"clients"`
	Disk     config.Section           `koanf:"disk"`
	Parent   *webhookConfig           `koanf:"parent"`
	Missing  *webhookConfig           `koanf:"missing"`
	Untagged int
	Skipped  string `koanf:"-"`
	hidden   string
}

func TestRedacted(t *testing.T) {
	t.Parallel()

	t.Run("masks secrets wherever they sit", func(t *testing.T) {
		t.Parallel()
		cfg := &redactedConfig{
			Token:   "t0ken",
			Region:  "eu",
			Hooks:   []webhookConfig{{URL: "https://a", Secret: "s1"}},
			Clients: map[string]webhookConfig{"crm": {URL: "https://b", Secret: "s2"}},
			Disk: config.Section{
				"driver":            "s3",
				"access_key_id":     "AKIA",
				"secret_access_key": "",
				"secret":            "sec",
				"keyspace":          "cache",
				"monkey":            "banana",
				"options": map[string]any{
					"API_KEY":      "k",
					"password":     "p",
					"database_dsn": "host=db",
					"credentials":  map[string]any{"user": "u"},
					"signing_key":  "s",
					"private_key":  "pk",
					"auth_token":   "t",
					"empty_token":  nil,
				},
			},
			Parent:   &webhookConfig{URL: "https://c", Secret: "s3"},
			Untagged: 3,
			Skipped:  "x",
			hidden:   "y",
		}

		got := config.Redacted(cfg)

		assert.Equal(t, map[string]any{
			"token":   "[redacted]",
			"unset":   "",
			"region":  "eu",
			"hooks":   []any{map[string]any{"url": "https://a", "secret": "[redacted]"}},
			"clients": map[string]any{"crm": map[string]any{"url": "https://b", "secret": "[redacted]"}},
			"disk": map[string]any{
				"driver":            "s3",
				"access_key_id":     "AKIA",
				"secret_access_key": "",
				"secret":            "[redacted]",
				"keyspace":          "cache",
				"monkey":            "banana",
				"options": map[string]any{
					"API_KEY":      "[redacted]",
					"password":     "[redacted]",
					"database_dsn": "[redacted]",
					"credentials":  "[redacted]",
					"signing_key":  "[redacted]",
					"private_key":  "[redacted]",
					"auth_token":   "[redacted]",
					"empty_token":  nil,
				},
			},
			"parent":   map[string]any{"url": "https://c", "secret": "[redacted]"},
			"missing":  nil,
			"Untagged": 3,
		}, got)
	})

	t.Run("gives an empty map for values that are not structs or maps", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, map[string]any{}, config.Redacted(42))
		assert.Equal(t, map[string]any{}, config.Redacted(nil))
	})
}

func TestRedactCredentials(t *testing.T) {
	t.Parallel()

	t.Run("masks the password of a connection url and keeps the rest", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "postgresql://svc_app:[redacted]@db.internal:5432/main?sslmode=verify-full",
			config.RedactCredentials("postgresql://svc_app:s3cr3t@db.internal:5432/main?sslmode=verify-full"))
		assert.Equal(t, "rediss://:[redacted]@cache.internal:6379",
			config.RedactCredentials("rediss://:tok3n@cache.internal:6379"))
		assert.Equal(t, "postgres://user:[redacted]@host/db",
			config.RedactCredentials("postgres://user:p@ss:word@host/db"))
	})

	t.Run("masks a password pair in a key value connection string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "host=db user=app password=[redacted] dbname=main",
			config.RedactCredentials("host=db user=app password=s3cr3t dbname=main"))
		assert.Equal(t, "host=db password=[redacted]", config.RedactCredentials("host=db password='a b'"))
	})

	t.Run("leaves values without a password alone", func(t *testing.T) {
		t.Parallel()

		for _, value := range []string{
			"https://staging.example.com", "postgres://user@host/db", "redis://host:6379", "plain", "",
			"https://example.com/path?next=a@b",
		} {
			assert.Equal(t, value, config.RedactCredentials(value))
		}
	})

	t.Run("redacts connection strings anywhere in the configuration", func(t *testing.T) {
		t.Parallel()
		out := config.Redacted(&struct {
			URL  string   `koanf:"url"`
			Read []string `koanf:"read_urls"`
		}{URL: "postgres://u:" + "p1@h/db", Read: []string{"postgres://u:" + "p2@r/db"}})

		assert.Equal(t, "postgres://u:[redacted]@h/db", out["url"])
		assert.Equal(t, []any{"postgres://u:[redacted]@r/db"}, out["read_urls"])
	})
}
