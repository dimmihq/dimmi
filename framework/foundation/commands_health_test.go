package foundation_test

import (
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
)

func TestHealthCommand(t *testing.T) {
	t.Parallel()

	t.Run("a worker answers health on its admin listener and the command sees it ready", func(t *testing.T) {
		t.Parallel()
		env := environ(t, "APP_APP__ROLES=worker,scheduler")
		app := newApp(t, options(env))
		require.NoError(t, app.Start(t.Context()))

		require.NotEmpty(t, app.AdminAddr())
		assert.Empty(t, app.Addr())
		assert.Equal(t, http.StatusOK, mustGet(t, "http://"+app.AdminAddr()+foundation.PathReady).Status)

		code, stdout, _ := run(t, options(append(env, "APP_HTTP__ADMIN_ADDR="+app.AdminAddr())), "", "health")
		assert.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "ready")
	})

	t.Run("fails when nothing answers", func(t *testing.T) {
		t.Parallel()
		listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		closed := listener.Addr().String()
		require.NoError(t, listener.Close())

		code, _, stderr := run(t, options(environ(t, "APP_HTTP__ADMIN_ADDR="+closed)), "", "health")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "not ready")
	})

	t.Run("fails without an admin listener to ask", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, options(environ(t, "APP_HTTP__ADMIN_ADDR=")), "", "health")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "http.admin_addr is empty")
	})
}
