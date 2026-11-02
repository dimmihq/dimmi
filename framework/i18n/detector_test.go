package i18n_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/manager"
)

type fixed string

func (f fixed) Detect(string) (string, float64) { return string(f), 1 }

func TestDetectorManager(t *testing.T) {
	t.Parallel()

	newManager := func() *i18n.DetectorManager {
		m := i18n.NewDetectorManager(i18n.DetectorConfig{
			Default: "text",
			Detectors: map[string]config.Section{
				"text":  {"driver": "fixed", "language": "fr"},
				"other": {"driver": "missing"},
			},
		})
		m.Extend("fixed", func(_ string, section config.Section) (i18n.Detector, error) {
			return fixed(section.String("language")), nil
		})

		return m
	}

	t.Run("builds the default detector from its section", func(t *testing.T) {
		t.Parallel()

		detector, err := newManager().Default()
		require.NoError(t, err)

		got, confidence := detector.Detect("bonjour")
		assert.Equal(t, "fr", got)
		assert.InDelta(t, 1.0, confidence, 0)
	})

	t.Run("resolves detectors by name", func(t *testing.T) {
		t.Parallel()
		m := newManager()

		_, err := m.Detector("text")
		require.NoError(t, err)

		_, err = m.Detector("other")
		require.ErrorIs(t, err, manager.ErrUnknownDriver)
	})
}
