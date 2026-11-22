package pipeline

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/models"
)

func TestAgentPurpose(t *testing.T) {
	t.Run("reads who the agent talks to from its guidance, without headings", func(t *testing.T) {
		got := agentPurpose(models.AiAgent{Guidance: "## Who you talk to\n\nPeople who have not bought yet. " +
			"Be consultative, never pushy.\n\n## Qualifying\n\nLearn company size."})

		assert.True(t, strings.HasPrefix(got, "People who have not bought yet."), got)
		assert.NotContains(t, got, "#")
		assert.NotContains(t, got, "Who you talk to")
	})

	t.Run("stops at about a paragraph", func(t *testing.T) {
		got := agentPurpose(models.AiAgent{Guidance: strings.Repeat("Answer billing questions by policy. ", 30)})

		assert.LessOrEqual(t, len(got), purposeLength+len("…"))
	})

	t.Run("says nothing for an agent without guidance", func(t *testing.T) {
		assert.Empty(t, agentPurpose(models.AiAgent{}))
	})
}
