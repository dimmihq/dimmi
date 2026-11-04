package arch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
TestLocales reads app.locales as a release ships it, from its defaults
alone, so a development config listing the pseudo locale never counts.
*/
func TestLocales(t *testing.T) {
	t.Parallel()

	t.Run("every locale a release serves has server and browser messages", func(t *testing.T) {
		t.Parallel()

		app, err := config.Load[foundation.AppConfig](t.Context(), config.LoadOptions{Name: "app"})
		require.NoError(t, err)
		require.NotEmpty(t, app.Locales)

		root := filepath.Join("..", "..")
		for _, locale := range app.Locales {
			for _, dir := range []string{"lang", filepath.Join("resources", "js", "lang")} {
				info, err := os.Stat(filepath.Join(root, dir, locale))
				if assert.NoError(t, err, "%s has no %s directory", locale, dir) {
					assert.True(t, info.IsDir(), "%s in %s is not a directory", locale, dir)
				}
			}
		}
	})

	t.Run("a release never serves the pseudo locale", func(t *testing.T) {
		t.Parallel()

		app, err := config.Load[foundation.AppConfig](t.Context(), config.LoadOptions{Name: "app"})
		require.NoError(t, err)
		assert.NotContains(t, app.Locales, "en-XA")
	})
}
