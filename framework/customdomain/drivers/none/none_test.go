package none_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/none"
	"github.com/dimmihq/dimmi/framework/customdomain/drivertest"
)

func TestDriver(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T, authorize customdomain.Authorizer) customdomain.TLS {
		t.Helper()

		driver, err := none.Factory(authorize)("proxy", config.Section{"driver": "none"})
		require.NoError(t, err)

		return driver
	})

	t.Run("leaves tls to the proxy", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, none.New(drivertest.Authorize).TLSConfig())
	})
}
