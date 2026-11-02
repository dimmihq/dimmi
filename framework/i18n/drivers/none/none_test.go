package none_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/i18n/drivers/none"
	"github.com/dimmihq/dimmi/framework/i18n/drivertest"
)

func TestDetector(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, func(t *testing.T) i18n.Detector {
		t.Helper()

		detector, err := none.Factory("off", config.Section{"driver": "none"})
		require.NoError(t, err)

		return detector
	})
}
