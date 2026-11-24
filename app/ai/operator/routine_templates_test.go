package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutineTemplatesNotify(t *testing.T) {
	t.Parallel()

	t.Run("tells the team after every run of a routine that only reports", func(t *testing.T) {
		t.Parallel()

		for _, tpl := range conversationTemplates() {
			assert.Equal(t, "always", tpl.NotifyPolicy, tpl.Key)
		}
	})
}
