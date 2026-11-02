package caddyask_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/caddyask"
	"github.com/dimmihq/dimmi/framework/customdomain/drivertest"
)

func TestDriver(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T, authorize customdomain.Authorizer) customdomain.TLS {
		t.Helper()

		driver, err := caddyask.Factory(authorize)("edge", config.Section{"driver": "caddy_ask"})
		require.NoError(t, err)

		return driver
	})

	t.Run("leaves tls to caddy", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, caddyask.New(drivertest.Authorize).TLSConfig())
	})
}

func TestAskHandler(t *testing.T) {
	t.Parallel()

	ask := func(t *testing.T, query string) int {
		t.Helper()

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/internal/tls/ask?"+query, http.NoBody)
		caddyask.New(drivertest.Authorize).AskHandler().ServeHTTP(recorder, request)

		return recorder.Code
	}

	t.Run("allows an authorized hostname", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, ask(t, "domain="+drivertest.Host))
	})

	t.Run("normalizes the hostname before asking", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusOK, ask(t, "domain=Docs.Example.com."))
	})

	t.Run("refuses a hostname the authorizer refuses", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusNotFound, ask(t, "domain="+drivertest.Refused))
	})

	t.Run("refuses a missing or invalid hostname", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, http.StatusNotFound, ask(t, ""))
		assert.Equal(t, http.StatusNotFound, ask(t, "domain=127.0.0.1"))
	})
}
