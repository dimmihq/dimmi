package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app"
	appconfig "github.com/dimmihq/dimmi/config"
)

func TestSessionsSources(t *testing.T) {
	t.Parallel()

	sources := func(host, key string) []string {
		frontend := appconfig.FrontendConfig{SessionsHost: host, SessionsKey: key}

		return sessionsSources(&app.App{Config: &appconfig.Config{Frontend: frontend}})
	}

	t.Run("allows the recorder's origin", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{"https://sessions.example.com"}, sources("https://sessions.example.com/", "km_pub_0123abcd"))
	})

	t.Run("allows nothing without a recorder", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, sources("", ""))
		assert.Empty(t, sources("https://sessions.example.com", ""))
	})
}
