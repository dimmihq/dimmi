package ai_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
transferDesk is a desk with two live agents: the default one, and Sales,
which answers the conversation first.
*/
type transferDesk struct {
	*harness
	support ids.UUID
	sales   ids.UUID
}

/*
newTransferDesk makes Sales live on the default agent's model and lets
it hand conversations to the default agent.
*/
func newTransferDesk(t *testing.T) *transferDesk {
	t.Helper()
	h := newHarness(t, nil)
	r := &transferDesk{harness: h}
	h.d.TC.InWorkspace(func(ctx context.Context) error {
		r.support = agentID(ctx, t, h.d)
		return nil
	})
	public := h.d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
		AssertCreated().String("data.agent.id")
	sales, err := h.d.TC.App.IDs.Decode(string(models.PrefixAiAgent), public)
	require.NoError(t, err)
	r.sales = sales
	h.operator(`UPDATE ai_agents s SET status = 'live', enabled = true, answer_key_id = d.answer_key_id,
		answer_model = d.answer_model, enabled_channels = d.enabled_channels, transfer_agent_ids = ARRAY[d.id]
		FROM ai_agents d WHERE s.id = $1 AND d.id = $2`, sales, r.support)
	return r
}

/*
allowBack lets the default agent hand conversations to Sales too.
*/
func (r *transferDesk) allowBack() {
	r.operator(`UPDATE ai_agents SET transfer_agent_ids = ARRAY[$1::uuid] WHERE id = $2`, r.sales, r.support)
}

/*
start opens a conversation Sales holds.
*/
func (r *transferDesk) start(body string) (ids.UUID, ids.UUID) {
	conv, trigger := r.conversation(body)
	r.operator(`UPDATE conversations SET ai_agent_id = $1 WHERE id = $2`, r.sales, conv)
	return conv, trigger
}

func TestAiAgentTransfers(t *testing.T) {
	t.Parallel()

	t.Run("hands the conversation over silently and the other agent answers", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Existing customer with a bug"}`)),
			reply("Sorry about the export. Which file type were you exporting?"),
		)
		conv, trigger := r.start("We're customers and the invoice export is broken")

		res := r.run(conv, trigger)

		assert.NotEqual(t, "transferred", res.Outcome, "the run returned is the one that answered")
		s := r.load(conv, res.RunID)
		require.Len(t, s.agentReplies(), 1, "the customer gets one reply, from the agent it went to")
		assert.Equal(t, "Sorry about the export. Which file type were you exporting?", *s.agentReplies()[0].Body)
		assert.Equal(t, r.support, *s.agentReplies()[0].AuthorID)
		events := strings.Join(s.timeline(), "\n")
		assert.Contains(t, events, `"name": "ai.agent_transferred"`)
		assert.Contains(t, events, `"summary": "Existing customer with a bug"`)
		require.NotNil(t, s.conv.AiAgentID)
		assert.Equal(t, r.support, *s.conv.AiAgentID, "the conversation stays with the agent it went to")
		assert.Equal(t, []ids.UUID{r.sales}, s.conv.AiTransferredFromIds)
		require.Len(t, s.runs, 2)
		outcomes := []string{*s.runs[0].Outcome, *s.runs[1].Outcome}
		assert.Contains(t, outcomes, "transferred")
	})

	t.Run("never hands a conversation back to an agent it left", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		r.allowBack()
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Billing bug"}`)),
			reply("Sorry about that."),
		)
		conv, trigger := r.start("Billing bug")

		r.run(conv, trigger)

		sent := r.llm.answer.Sent()
		require.Len(t, sent, 2)
		assert.Contains(t, offered(r.llm.answer, 0), "transfer_agent:", "Sales may transfer")
		assert.NotContains(t, offered(r.llm.answer, 1), "transfer_agent:", "the default agent may not send it back")
	})

	t.Run("offers no transfer to an agent that is not live", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		r.operator(`UPDATE ai_agents SET status = 'paused', enabled = false WHERE id = $1`, r.support)
		r.operator(`UPDATE ai_agents SET is_default = false WHERE id = $1`, r.support)
		r.llm.answer.Script(reply("Happy to help with pricing."))
		conv, trigger := r.start("How much is it?")

		r.run(conv, trigger)

		assert.NotContains(t, offered(r.llm.answer, 0), "transfer_agent:")
	})
}
