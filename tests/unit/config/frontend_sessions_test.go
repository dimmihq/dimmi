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
