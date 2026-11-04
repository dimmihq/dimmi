package i18n_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/i18n"
)

func TestCanonical(t *testing.T) {
	t.Parallel()

	t.Run("spells a tag the canonical way", func(t *testing.T) {
		t.Parallel()

		got, ok := i18n.Canonical("PT-br")

		assert.True(t, ok)
		assert.Equal(t, "pt-BR", got)
	})

	t.Run("refuses text that is no tag", func(t *testing.T) {
		t.Parallel()

		_, ok := i18n.Canonical("not a tag")

		assert.False(t, ok)
	})
}
