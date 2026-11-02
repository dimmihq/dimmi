package whatlang_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/i18n/drivers/whatlang"
	"github.com/dimmihq/dimmi/framework/i18n/drivertest"
)

func newDetector(t *testing.T) i18n.Detector {
	t.Helper()

	detector, err := whatlang.Factory("text", config.Section{"driver": "whatlang"})
	require.NoError(t, err)

	return detector
}

func TestDetector(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newDetector)

	t.Run("detects english with confidence", func(t *testing.T) {
		t.Parallel()

		got, confidence := newDetector(t).Detect("The weather is lovely today and we are going to the beach.")

		assert.Equal(t, "en", got)
		assert.Greater(t, confidence, 0.5)
	})

	t.Run("names a language without a two letter code by its three letter code", func(t *testing.T) {
		t.Parallel()

		cebuano := "Ang Sinugboanon usa ka pinulongan nga gisulti sa mga tawo sa Sugbo ug sa Bisayas"

		got, _ := newDetector(t).Detect(cebuano)

		assert.Equal(t, "ceb", got)
	})
}
