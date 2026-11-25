package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	frameworkconfig "github.com/dimmihq/dimmi/framework/config"

	"github.com/dimmihq/dimmi/config"
)

func TestSlackDataDeletionGraceDays(t *testing.T) {
	t.Parallel()

	load := func(t *testing.T, values settings) (config.Config, error) {
		t.Helper()

		var cfg config.Config

		err := frameworkconfig.LoadInto(t.Context(), frameworkconfig.LoadOptions{Runtime: values}, &cfg)

		return cfg, err
	}

	t.Run("defaults to ten days", func(t *testing.T) {
		t.Parallel()

		cfg, err := load(t, settings{})
		require.NoError(t, err)
		assert.Equal(t, 10, cfg.Channels.Slack.DataDeletionGraceDays)
	})

	t.Run("accepts the fourteen days Slack allows", func(t *testing.T) {
		t.Parallel()

		cfg, err := load(t, settings{"channels.slack.data_deletion_grace_days": 14})
		require.NoError(t, err)
		assert.Equal(t, 14, cfg.Channels.Slack.DataDeletionGraceDays)
	})

	t.Run("refuses more than fourteen days, and a negative period", func(t *testing.T) {
		t.Parallel()

		_, err := load(t, settings{"channels.slack.data_deletion_grace_days": 15})
		require.ErrorIs(t, err, config.ErrInvalid)

		_, err = load(t, settings{"channels.slack.data_deletion_grace_days": -1})
		require.ErrorIs(t, err, config.ErrInvalid)
	})
}
