package image_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/image"
)

func TestThumbnail(t *testing.T) {
	t.Parallel()

	t.Run("makes a square png of the size from any accepted picture", func(t *testing.T) {
		t.Parallel()

		for _, name := range []string{"logo.png", "logo.webp", "animated.gif", "orientation_1.jpg"} {
			out, cfg, err := image.Thumbnail(bytes.NewReader(fixture(t, name)), 64, image.Limits{})
			require.NoError(t, err, name)
			assert.NotEmpty(t, cfg.Format, name)

			made, err := image.DecodeConfig(bytes.NewReader(out), image.Limits{})
			require.NoError(t, err, name)
			assert.Equal(t, image.PNG, made.Format, name)
			assert.Equal(t, 64, made.Width, name)
			assert.Equal(t, 64, made.Height, name)
		}
	})

	t.Run("refuses what is not a picture and what is over the limits", func(t *testing.T) {
		t.Parallel()

		_, _, err := image.Thumbnail(bytes.NewReader(fixture(t, "image.svg")), 64, image.Limits{})
		require.ErrorIs(t, err, image.ErrUnsupportedFormat)

		_, _, err = image.Thumbnail(bytes.NewReader(fixture(t, "logo.png")), 64, image.Limits{MaxBytes: 4})
		require.ErrorIs(t, err, image.ErrTooLarge)
	})
}
