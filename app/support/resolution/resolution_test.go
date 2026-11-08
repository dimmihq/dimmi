package resolution_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/resolution"
)

func TestConfirms(t *testing.T) {
	t.Parallel()

	t.Run("accepts short confirmations however they are written", func(t *testing.T) {
		t.Parallel()

		for _, text := range []string{
			"yes", "Yes!", "  that helped ", "Thanks!!", "thank you so much", "Perfect, thanks",
		} {
			assert.True(t, resolution.Confirms(text), text)
		}
	})

	t.Run("refuses questions, denials and new problems", func(t *testing.T) {
		t.Parallel()

		for _, text := range []string{
			"yes?", "no", "not really", "thanks but it still fails", "it didn't work", "yes, and how do I export?",
			"", "I need to change my billing address please",
		} {
			assert.False(t, resolution.Confirms(text), text)
		}
	})
}
