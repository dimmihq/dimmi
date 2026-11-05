package foundation_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/http/middleware"
)

func TestGateSetting(t *testing.T) {
	t.Parallel()

	t.Run("closes the routes, not health, when enabled", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_GATE__ENABLED=true", "APP_GATE__PASSWORD=shared", "APP_GATE__CI_KEY=key"))
		opts.GateExcept = []string{"/open/*"}
		app := newApp(t, opts)
		require.NoError(t, app.Start(t.Context()))
		base := "http://" + app.Addr()

		code, _ := plainGet(t, app.Addr(), "localhost", "/anything")
		assert.Equal(t, http.StatusUnauthorized, code)
		code, _ = plainGet(t, app.Addr(), "localhost", "/open/thing")
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, http.StatusOK, mustGet(t, base+foundation.PathReady).Status)
		assert.Equal(t, http.StatusOK, mustGet(t, base+"/robots.txt").Status)
	})

	t.Run("is off unless enabled", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))

		code, _ := plainGet(t, app.Addr(), "localhost", "/anything")
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("refuses to boot enabled with no way in", func(t *testing.T) {
		t.Parallel()

		_, err := foundation.New(t.Context(), options(environ(t, "APP_GATE__ENABLED=true")))

		require.ErrorContains(t, err, "gate.password")
	})

	t.Run("names the key header the deploy smoke test sends", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "X-Gate-Key", middleware.GateKeyHeader)
	})
}
