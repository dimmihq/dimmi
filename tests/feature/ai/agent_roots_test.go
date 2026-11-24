package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

func TestAiAgentRoots(t *testing.T) {
	t.Parallel()

	t.Run("names the root of each template", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, prompts.RootSales, prompts.RootFor("sales"))
		for _, key := range []string{"", "support", "billing", "onboarding", "internal", "no-such-template"} {
			assert.Equal(t, prompts.RootSupport, prompts.RootFor(key), key)
		}
	})

	t.Run("an agent made from the sales template starts with the guidance written for it", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		written, ok := prompts.TemplateGuidance("sales")
		require.True(t, ok)

		made := h.d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
			AssertCreated()

		made.AssertJSONPath("data.agent.draft.guidance", written)
		assert.Contains(t, written, "## When they are not interested",
			"every decision the root leaves to the team is made in it, ready to edit")
	})

	t.Run("an agent made from the sales template answers from the sales root", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Existing customer with a bug"}`)),
			reply("Sorry about the export. Which file type were you exporting?"),
		)
		conv, trigger := r.start("We're customers and the invoice export is broken")

		r.run(conv, trigger)

		sent := r.llm.answer.Sent()
		require.Len(t, sent, 2)
		sales, held := systemOf(sent[0]), systemOf(sent[1])
		assert.Contains(t, sales, "sales team, talking with people who are deciding whether to buy")
		assert.Contains(t, sales, "You never make one of the team's decisions for them.")
		assert.Contains(t, sales, "# Other agents", "what is layered on a root is layered on this one too")
		assert.NotContains(t, sales, "{{", "every placeholder is filled")
		assert.Contains(t, held, "support team")
		assert.NotContains(t, held, "You never make one of the team's decisions for them.")
	})
}
