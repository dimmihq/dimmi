package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app"
	appconfig "github.com/dimmihq/dimmi/config"
)

func TestErrorsSources(t *testing.T) {
	t.Parallel()

	sources := func(dsn string) []string {
		return errorsSources(&app.App{Config: &appconfig.Config{Frontend: appconfig.FrontendConfig{ErrorsDSN: dsn}}})
	}

	t.Run("allows the tracker's origin without the project's key", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"https://errors.example.com"}, sources("https://0123abcd@errors.example.com/2"))
	})

	t.Run("allows nothing without a dsn", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, sources(""))
	})
}

func TestTrafficScriptSources(t *testing.T) {
	t.Parallel()

	sources := func(edition appconfig.Edition, dsn string) []string {
		return trafficScriptSources(&app.App{Config: &appconfig.Config{
			App:      appconfig.AppConfig{Edition: edition},
			Frontend: appconfig.FrontendConfig{ErrorsDSN: dsn},
		}})
	}

	t.Run("allows the tracker origin on cloud builds", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"https://errors.example.com"},
			sources(appconfig.EditionCloud, "https://0123abcd@errors.example.com/2"))
	})

	t.Run("allows nothing on self-hosted builds", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, sources(appconfig.EditionSelfHost, "https://0123abcd@errors.example.com/2"))
	})
}
