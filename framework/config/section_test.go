package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
)

type diskOptions struct {
	Root      string `koanf:"root"`
	Region    string `default:"us-east-1" koanf:"region"`
	PathStyle bool   `koanf:"force_path_style"`
	Retries   int    `default:"3"         koanf:"retries"`
}

func TestSection(t *testing.T) {
	t.Parallel()

	t.Run("decodes values over defaults", func(t *testing.T) {
		t.Parallel()
		section := config.Section{"driver": "s3", "root": "/srv", "force_path_style": true, "retries": 5}

		var opts diskOptions
		require.NoError(t, section.Decode(&opts))

		assert.Equal(t, diskOptions{Root: "/srv", Region: "us-east-1", PathStyle: true, Retries: 5}, opts)
	})

	t.Run("splits a string given for a list option", func(t *testing.T) {
		t.Parallel()

		var opts struct {
			Hosts   []string `koanf:"hosts"`
			Regions []string `koanf:"regions"`
		}
		section := config.Section{"hosts": "a.example.com, b.example.com", "regions": []any{"eu,us"}}
		require.NoError(t, section.Decode(&opts))

		assert.Equal(t, []string{"a.example.com", "b.example.com"}, opts.Hosts)
		assert.Equal(t, []string{"eu,us"}, opts.Regions)
	})

	t.Run("reports values of the wrong type", func(t *testing.T) {
		t.Parallel()

		var opts diskOptions
		err := config.Section{"retries": []string{"x"}}.Decode(&opts)

		require.ErrorContains(t, err, "decode section")
	})

	t.Run("reports a target defaults cannot fill", func(t *testing.T) {
		t.Parallel()

		var notStruct int
		require.ErrorContains(t, config.Section{}.Decode(&notStruct), "apply defaults")
	})

	t.Run("reads string options", func(t *testing.T) {
		t.Parallel()
		section := config.Section{"driver": "local", "retries": 3}

		assert.Equal(t, "local", section.String("driver"))
		assert.Empty(t, section.String("retries"))
		assert.Empty(t, section.String("missing"))
	})
}
