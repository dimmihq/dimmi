package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	frameworkconfig "github.com/dimmihq/dimmi/framework/config"

	"github.com/dimmihq/dimmi/config"
)

func TestFrontendSessionRecorder(t *testing.T) {
	t.Parallel()

	load := func(t *testing.T, values settings) (config.Config, error) {
		t.Helper()

		var cfg config.Config

		err := frameworkconfig.LoadInto(t.Context(), frameworkconfig.LoadOptions{Runtime: values}, &cfg)

		return cfg, err
	}

	t.Run("records nothing by default", func(t *testing.T) {
		t.Parallel()

		cfg, err := load(t, settings{})
		require.NoError(t, err)
		assert.False(t, cfg.Frontend.Records())
		assert.Empty(t, cfg.Frontend.SessionsOrigin())
	})

	t.Run("records on the recorder a host and key name", func(t *testing.T) {
		t.Parallel()

		cfg, err := load(t, settings{
			"frontend.sessions_host": "https://sessions.example.com/",
			"frontend.sessions_key":  "km_pub_0123abcd",
		})
		require.NoError(t, err)
		assert.True(t, cfg.Frontend.Records())
		assert.Equal(t, "https://sessions.example.com", cfg.Frontend.SessionsOrigin())
	})

	t.Run("erases recordings only with a recorder, a project id and a token", func(t *testing.T) {
		t.Parallel()

		recorder := settings{
			"frontend.sessions_host":       "https://sessions.example.com/",
			"frontend.sessions_key":        "km_pub_0123abcd",
			"frontend.sessions_project_id": "0193a0b1-0000-7000-8000-000000000001",
			"frontend.sessions_token":      "km_sec_0123abcd",
		}

		cfg, err := load(t, recorder)
		require.NoError(t, err)
		assert.Equal(t,
			"https://sessions.example.com/api/v1/projects/0193a0b1-0000-7000-8000-000000000001/visitors/erase",
			cfg.Frontend.SessionsEraseURL())

		for _, missing := range []string{"frontend.sessions_project_id", "frontend.sessions_token"} {
			partial := settings{}
			for k, v := range recorder {
				if k != missing {
					partial[k] = v
				}
			}

			cfg, err := load(t, partial)
			require.NoError(t, err, missing)
			assert.Empty(t, cfg.Frontend.SessionsEraseURL(), missing)
		}

		cfg, err = load(t, settings{"frontend.sessions_project_id": "p", "frontend.sessions_token": "km_sec_0123abcd"})
		require.NoError(t, err)
		assert.Empty(t, cfg.Frontend.SessionsEraseURL())
	})

	t.Run("refuses a host without a key, a key without a host, and a host that is not an origin", func(t *testing.T) {
		t.Parallel()

		for _, values := range []settings{
			{"frontend.sessions_host": "https://sessions.example.com"},
			{"frontend.sessions_key": "km_pub_0123abcd"},
			{"frontend.sessions_host": "sessions.example.com", "frontend.sessions_key": "km_pub_0123abcd"},
			{"frontend.sessions_host": "https://sessions.example.com/kimmi.js", "frontend.sessions_key": "km_pub_0123abcd"},
			{"frontend.sessions_host": "javascript:alert(1)", "frontend.sessions_key": "km_pub_0123abcd"},
		} {
			_, err := load(t, values)
			require.ErrorIs(t, err, config.ErrInvalid, values)
		}
	})
}
