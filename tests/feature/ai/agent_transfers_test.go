package ai_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	decisionfake "github.com/dimmihq/dimmi/app/drivers/decision/fake"
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
		var counted int64
		for _, run := range s.runs {
			if *run.Outcome == "transferred" || *run.Outcome == "answered" {
				counted += run.CostMicros
			}
		}
		assert.Positive(t, counted)
		assert.Equal(t, counted, s.conv.AiCostMicros, "the conversation's cost counts the transfer's turn too")
	})

	t.Run("the inbox lists the conversations moved between agents", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Existing customer"}`)),
			reply("Happy to help with that."),
		)
		moved, trigger := r.start("We're customers and need help")
		r.run(moved, trigger)
		r.conversation("Just browsing")

		res := r.d.Owner.GetJSON("/api/v1/conversations?filter[view]=all&filter[status]=any&filter[ai]=transferred").
			AssertOK()
		res.AssertJSONPath("data.#", 1)
		res.AssertJSONPath("data.0.id", models.PublicID(models.PrefixConversation, moved))
	})

	t.Run("the team sees which agent wrote a reply", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		r.llm.answer.Script(reply("Growth fits 500 labels a month."))
		conv, trigger := r.start("Which plan fits 500 labels?")

		r.run(conv, trigger)

		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conv) + "/messages"
		var page struct {
			Data []struct {
				Author struct {
					Type string `json:"type"`
					Name string `json:"name"`
				} `json:"author"`
			} `json:"data"`
		}
		r.d.Owner.GetJSON(path).AssertOK().Decode(&page)
		var names []string
		for _, m := range page.Data {
			if m.Author.Type == "agent" {
				names = append(names, m.Author.Name)
			}
		}
		assert.Equal(t, []string{"Sales"}, names)
	})

	t.Run("a simulation stays with the agent it was transferred to, and its judge reads who answered", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		r.allowBack()
		d := r.decider()
		d.Script(decisionfake.Step{Answers: map[string]contracts.DecisionAnswer{
			"met": {Type: contracts.DecisionNoul, Noul: 0.94},
		}})
		sales := models.PublicID(models.PrefixAiAgent, r.sales)
		path := "/api/v1/ai/agents/" + models.PublicID(models.PrefixAiAgent, r.support) + "/simulations"
		id := r.d.Owner.PostJSON(path, testkit.JSON{
			"name":       "Pricing goes to sales",
			"messages":   []testkit.JSON{{"text": "Which plan fits 500 labels?"}, {"text": "About 20 a day"}},
			"assertions": []testkit.JSON{{"type": "llm_rubric", "rubric": "Stays with the agent it was transferred to"}},
		}).AssertCreated().String("data.id")
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+sales+`","summary":"Prospect asking about plans"}`)),
			reply("Happy to help. How many a day?"),
			reply("Growth fits that."),
		)

		ran := r.d.Owner.PostJSON(path+"/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()

		ran.AssertJSONPath("data.0.draft_result.passed", true)
		ran.AssertJSONPath("data.0.draft_result.turns.1.reply", "Growth fits that.")
		ran.AssertJSONPath("data.0.draft_result.turns.0.transfer.name", "Sales")
		ran.AssertJSONPath("data.0.draft_result.turns.0.transfer.summary", "Prospect asking about plans")
		require.Len(t, r.llm.answer.Sent(), 3, "the second message goes straight to the agent holding the chat")
		var state string
		for _, asked := range d.Sent() {
			if _, ok := asked.Questions["met"]; ok {
				state = asked.State
			}
		}
		assert.Contains(t, state, "CUSTOMER: Which plan fits 500 labels?\n[")
		assert.Contains(t, state, " transferred the conversation to Sales. Its note for Sales: Prospect asking about plans]")
		assert.Contains(t, state, "SALES: Happy to help. How many a day?\nCUSTOMER: About 20 a day\nSALES: Growth fits that.")
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
		assert.Contains(t, systemOf(sent[0]), "# Other agents", "Sales is told who it may pass the chat to")
		assert.NotContains(t, systemOf(sent[1]), "# Other agents",
			"an agent that cannot transfer is told nothing about other agents")
	})

	t.Run("tells each agent its own job and what the agents it may pass to handle", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		r.allowBack()
		r.operator(`UPDATE ai_agents SET description = 'Existing customers with a problem in their account.'
			WHERE id = $1`, r.support)
		r.operator(`UPDATE ai_agents SET description = 'People who have not bought yet: prices and plans.'
			WHERE id = $1`, r.sales)
		r.llm.answer.Script(reply("Growth fits 500 labels a month."))
		conv, trigger := r.conversation("Which plan fits 500 labels?")

		r.run(conv, trigger)

		system := systemOf(r.llm.answer.Sent()[0])
		assert.Contains(t, system, "# Your job")
		assert.Contains(t, system, "Existing customers with a problem in their account.")
		assert.Contains(t, system, "# Other agents")
		assert.Contains(t, system, "- Sales (id "+models.PublicID(models.PrefixAiAgent, r.sales)+", sales agent): "+
			"People who have not bought yet: prices and plans.")
		assert.Contains(t, system, "Transfer even when you could answer the question yourself.")
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
