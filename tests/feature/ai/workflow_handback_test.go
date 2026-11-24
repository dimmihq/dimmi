package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
liveWorkflow publishes a workflow with the trigger and steps.
*/
func (h *harness) liveWorkflow(name, trigger string, steps ...testkit.JSON) string {
	h.t.Helper()
	spec := testkit.JSON{"schema": 1, "trigger": trigger, "steps": steps}
	id := h.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": name, "spec": spec}).
		AssertCreated().String("data.id")
	h.d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
		AssertOK()

	return id
}

/*
wfStep is one workflow step.
*/
func wfStep(id, kind string, settings testkit.JSON) testkit.JSON {
	return testkit.JSON{"id": id, "kind": kind, "settings": settings}
}

func TestWorkflowHandsBack(t *testing.T) {
	t.Parallel()

	t.Run("lets the agent answer the next message once a greeting workflow ends", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.liveWorkflow("Greeting", "conversation.first_message",
			wfStep("s_greet00001", "message", testkit.JSON{"text": "Hi! Thanks for writing in."}))
		h.llm.answer.Script(search("pro plan price per seat"), reply("The Pro plan costs $12 per seat per month [1]."))

		conv, _ := h.conversation("Hello there")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		assert.Empty(t, s.agentReplies(), "the workflow answered the first message")
		assert.Equal(t, "auto", s.conv.AiMode)
		assert.Nil(t, s.conv.WorkflowRunID)

		h.customer(conv, "How much is Pro per seat?")
		h.d.TC.DrainLater()

		s = h.load(conv, ids.UUID{})
		require.Len(t, s.agentReplies(), 1)
		assert.Contains(t, str(s.agentReplies()[0].Body), "$12 per seat")
	})

	t.Run("keeps every agent off once a workflow turns AI off, until one turns it on", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.liveWorkflow("Quiet please", "conversation.first_message", wfStep("s_off0000001", "turn_off_ai", testkit.JSON{}))
		h.liveWorkflow("Back on", "note.created", wfStep("s_on00000001", "turn_on_ai", testkit.JSON{}))
		h.llm.answer.Script(search("pro plan price per seat"), reply("The Pro plan costs $12 per seat per month [1]."))

		conv, _ := h.conversation("How much is Pro per seat?")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		assert.Empty(t, s.agentReplies())
		assert.Equal(t, "handed_off", s.conv.AiState)

		h.customer(conv, "Anyone?")
		h.d.TC.DrainLater()
		assert.Empty(t, h.load(conv, ids.UUID{}).agentReplies())

		h.d.Owner.PostJSON("/api/v1/conversations/"+models.PublicID(models.PrefixConversation, conv)+"/notes",
			testkit.JSON{"body": "Let the agent take it."}).AssertCreated()
		h.d.TC.Drain()
		assert.Equal(t, "off", h.load(conv, ids.UUID{}).conv.AiState)

		h.customer(conv, "How much is Pro per seat?")
		h.d.TC.DrainLater()
		assert.Len(t, h.load(conv, ids.UUID{}).agentReplies(), 1)
	})

	t.Run("puts the agent a workflow assigns on the conversation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		sales := h.d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
			AssertCreated().String("data.agent.id")
		h.liveWorkflow("Route to sales", "conversation.first_message",
			wfStep("s_assign0001", "assign_ai_agent", testkit.JSON{"agent_id": sales}))

		conv, _ := h.conversation("Can I get a demo?")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		require.NotNil(t, s.conv.AiAgentID)
		assert.Equal(t, sales, models.PublicID(models.PrefixAiAgent, *s.conv.AiAgentID))
	})

	t.Run("refuses to publish a step that names an agent the workspace does not have", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		spec := testkit.JSON{"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{
			wfStep("s_assign0001", "assign_ai_agent", testkit.JSON{"agent_id": models.PublicID(models.PrefixAiAgent, ids.New())}),
		}}
		id := h.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Nowhere", "spec": spec}).
			AssertCreated().String("data.id")
		h.d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertStatus(422)
	})

	t.Run("refuses the agent step's removed settings", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		spec := testkit.JSON{"schema": 1, "trigger": "conversation.first_message", "steps": []testkit.JSON{
			wfStep("s_dimmi00001", "dimmi", testkit.JSON{"instructions": "Only answer billing."}),
		}}
		id := h.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Old step", "spec": spec}).
			AssertCreated().String("data.id")
		h.d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
			AssertStatus(422)
	})
}

func TestAgentArchive(t *testing.T) {
	t.Parallel()

	t.Run("refuses to archive an agent a workflow or another agent still relies on", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		sales := h.d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Sales", "template": "sales"}).
			AssertCreated().String("data.agent.id")
		support := h.d.Owner.PostJSON("/api/v1/ai/agents", testkit.JSON{"name": "Support", "template": "support"}).
			AssertCreated().String("data.agent.id")
		h.d.Owner.PatchJSON("/api/v1/ai/agents/"+support, testkit.JSON{"transfer_agent_ids": []string{sales}}).AssertOK()
		wf := h.liveWorkflow("Route to sales", "conversation.first_message",
			wfStep("s_assign0001", "assign_ai_agent", testkit.JSON{"agent_id": sales}))

		view := h.d.Owner.GetJSON("/api/v1/ai/agents/" + sales).AssertOK()
		view.AssertJSONPath("data.ties.#", 2)
		view.AssertJSONPath("data.ties.0.kind", "agent")
		view.AssertJSONPath("data.ties.0.id", support)
		view.AssertJSONPath("data.ties.1.kind", "workflow")
		view.AssertJSONPath("data.ties.1.id", wf)

		refused := h.d.Owner.Delete("/api/v1/ai/agents/" + sales).AssertStatus(409)
		refused.AssertJSONPath("code", "agent_in_use")

		h.d.Owner.PatchJSON("/api/v1/ai/agents/"+support, testkit.JSON{"transfer_agent_ids": []string{}}).AssertOK()
		h.d.Owner.Delete("/api/v1/workflows/" + wf).AssertNoContent()
		h.d.Owner.GetJSON("/api/v1/ai/agents/"+sales).AssertOK().AssertJSONPath("data.ties.#", 0)
		h.d.Owner.Delete("/api/v1/ai/agents/" + sales).AssertNoContent()
		h.d.Owner.GetJSON("/api/v1/ai/agents/" + sales).AssertStatus(404)
	})
}

func TestWorkflowAgentTransfers(t *testing.T) {
	t.Parallel()

	t.Run("lets a named agent transfer when the step allows it, and starts the transfer trigger", func(t *testing.T) {
		t.Parallel()
		r := newTransferDesk(t)
		sales := models.PublicID(models.PrefixAiAgent, r.sales)
		support := models.PublicID(models.PrefixAiAgent, r.support)
		r.liveWorkflow("Sales first", "conversation.first_message", testkit.JSON{
			"id": "s_dimmi00001", "kind": "dimmi", "settings": testkit.JSON{"agent_id": sales, "allow_transfers": true},
		})
		r.liveWorkflow("Note transfers", "agent.transferred",
			wfStep("s_note000001", "note", testkit.JSON{"text": "Moved between agents"}))
		r.llm.answer.Script(
			tools(call("transfer_agent", `{"agent_id":"`+support+`","summary":"Existing customer with a bug"}`)),
			reply("Sorry about the export. Which file type were you exporting?"),
		)

		conv, _ := r.conversation("We're customers and the invoice export is broken")
		r.d.TC.Drain()

		s := r.load(conv, ids.UUID{})
		require.Len(t, s.agentReplies(), 1)
		assert.Equal(t, r.support, *s.agentReplies()[0].AuthorID, "the agent it went to answers")
		assert.Contains(t, noteTexts(s), "Moved between agents")
	})
}
