package customdomain_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/none"
	"github.com/dimmihq/dimmi/framework/customdomain/drivertest"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
newManager returns a manager whose default setup uses the driver named.
*/
func newManager(driver string) *customdomain.Manager {
	m := customdomain.NewManager(customdomain.Config{
		Default: "edge",
		TLS:     map[string]config.Section{"edge": {"driver": driver}},
	})
	m.Extend("none", none.Factory(drivertest.Authorize))

	return m
}

func TestManager(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(*testing.T, customdomain.Authorizer) customdomain.TLS { return newManager("none") })

	t.Run("resolves named setups", func(t *testing.T) {
		t.Parallel()
		m := newManager("none")

		named, err := m.Setup("edge")
		require.NoError(t, err)
		fallback, err := m.Default()
		require.NoError(t, err)

		assert.Same(t, named, fallback)
	})

	t.Run("fails every call when the default setup cannot be built", func(t *testing.T) {
		t.Parallel()
		m := newManager("unknown")
		next := http.NotFoundHandler()

		require.ErrorIs(t, m.Ensure(t.Context(), drivertest.Host), manager.ErrUnknownDriver)
		require.ErrorIs(t, m.Remove(t.Context(), drivertest.Host), manager.ErrUnknownDriver)
		assert.Nil(t, m.TLSConfig())

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		m.HTTPHandler(next).ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}
