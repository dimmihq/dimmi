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
