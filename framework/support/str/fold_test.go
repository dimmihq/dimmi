package str_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/support/str"
)

func TestFold(t *testing.T) {
	t.Parallel()

	t.Run("lowers case and composes letters", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "adéla", str.Fold("AdéLA"))
		assert.Equal(t, str.Fold("Café"), str.Fold("CAFÉ"))
	})

	t.Run("keeps accents", func(t *testing.T) {
		t.Parallel()

		assert.NotEqual(t, str.Fold("café"), str.Fold("cafe"))
	})

	t.Run("drops accents when asked", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "creme brulee", str.FoldAccents("Crème Brûlée"))
		assert.Equal(t, str.FoldAccents("Adéla"), str.FoldAccents("ADELA"))
	})

	t.Run("leaves scripts without marks as they are", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "שלום", str.FoldAccents("שלום"))
	})
}
