package ai_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/resolution"
)

/*
handlingWorkflow publishes a first-message workflow whose only step
lets dimmi handle the conversation, leaving a note on each path.
*/
func (h *harness) handlingWorkflow(settings testkit.JSON) {
	h.t.Helper()
	path := func(id, outcome, note string) testkit.JSON {
		return testkit.JSON{"id": id, "outcome": outcome, "steps": []testkit.JSON{{
			"id": "s_" + id[2:], "kind": "note", "settings": testkit.JSON{"text": note},
		}}}
	}
	spec := testkit.JSON{
		"schema": 1, "trigger": "conversation.first_message",
		"steps": []testkit.JSON{{
			"id": "s_dimmi00001", "kind": "dimmi", "settings": settings,
			"paths": []testkit.JSON{
				path("p_resolved01", "resolved", "path resolved"),
				path("p_handover01", "handed_over", "path handed"),
				path("p_quiet00001", "quiet", "path quiet"),
			},
		}},
	}
	id := h.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Let dimmi answer", "spec": spec}).
		AssertCreated().String("data.id")
	h.d.Owner.PostJSON("/api/v1/workflows/"+id+"/publications", testkit.JSON{"acknowledge_warnings": true}).
		AssertOK()
}

/*
customer writes on the conversation as its customer, through the API.
*/
func (h *harness) customer(conv ids.UUID, body string) {
	h.t.Helper()
	h.d.TC.Clock.Travel(time.Second)
	h.d.Owner.PostJSON("/api/v1/conversations/"+models.PublicID(models.PrefixConversation, conv)+"/messages",
		testkit.JSON{"body": body}).AssertCreated()
}

/*
noteTexts lists the bodies of the conversation's notes.
*/
func noteTexts(s state) []string {
	var out []string
	for _, n := range s.notes() {
		out = append(out, str(n.Body))
	}

	return out
}

func TestWorkflowDimmiStep(t *testing.T) {
	t.Parallel()

	t.Run("answers the first message, asks to confirm, and resolves on the customer's thanks", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.handlingWorkflow(testkit.JSON{"instructions": "Only answer questions about plans."})
		h.llm.answer.Script(search("pro plan price per seat"), reply("The Pro plan costs $12 per seat per month [1]."))

		conv, _ := h.conversation("How much is Pro per seat?")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		replies := s.agentReplies()
		require.Len(t, replies, 1)
		assert.True(t, strings.HasSuffix(str(replies[0].Body), resolution.Prompt), str(replies[0].Body))
		assert.Contains(t, sentText(h.llm.answer), "Only answer questions about plans.")
		assert.Equal(t, "workflow", s.conv.AiMode)

		h.customer(conv, "Thanks!")
		h.d.TC.DrainLater()

		s = h.load(conv, ids.UUID{})
		assert.Len(t, s.agentReplies(), 1)
		assert.Contains(t, noteTexts(s), "path resolved")
		assert.Equal(t, "off", s.conv.AiState)
	})

	t.Run("takes the handed-over path when the agent hands the conversation to the team", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		team := h.team("Refunds")
		h.operator(`UPDATE ai_agents SET handoff = jsonb_build_object('target_team_id', $2::uuid)
			WHERE workspace_id = $1`, h.d.TC.Workspace.Workspace.ID, team.ID)
		h.handlingWorkflow(testkit.JSON{})
		h.llm.answer.Script(handoffCall("refund"), reply("I'm bringing in a teammate."))

		conv, _ := h.conversation("You charged me twice. Refund one now.")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		assert.Equal(t, "handed_off", s.conv.AiState)
		assert.Contains(t, noteTexts(s), "path handed")
		assert.Nil(t, s.conv.AssigneeTeamID, "the workflow's path routes, not the agent's own setting")
	})

	t.Run("takes the quiet path once the customer says nothing after an answer", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.handlingWorkflow(testkit.JSON{"quiet_after": "15m"})
		h.llm.answer.Script(search("pro plan price per seat"), reply("The Pro plan costs $12 per seat per month [1]."))

		conv, _ := h.conversation("How much is Pro per seat?")
		h.d.TC.Drain()
		require.Len(t, h.load(conv, ids.UUID{}).agentReplies(), 1)

		h.d.TC.Clock.Travel(16 * time.Minute)
		h.d.TC.DrainLater()

		s := h.load(conv, ids.UUID{})
		assert.Contains(t, noteTexts(s), "path quiet")
		assert.Equal(t, "off", s.conv.AiState)
	})

	t.Run("keeps waiting when the customer writes something that is not a confirmation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.handlingWorkflow(testkit.JSON{})
		h.llm.answer.Script(search("pro plan price per seat"), reply("The Pro plan costs $12 per seat per month [1]."),
			search("annual discount"), reply("Paying yearly saves two months [1]."))

		conv, _ := h.conversation("How much is Pro per seat?")
		h.d.TC.Drain()
		h.customer(conv, "Thanks, but is there a discount for paying yearly?")
		h.d.TC.Clock.Travel(5 * time.Second)
		h.d.TC.DrainLater()

		s := h.load(conv, ids.UUID{})
		assert.Len(t, s.agentReplies(), 2)
		assert.NotContains(t, noteTexts(s), "path resolved")
		assert.Equal(t, "active", s.conv.AiState)
	})

	t.Run("hands over at once when the agent is off", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, func(p *models.UpdateAgentParams) { p.Enabled = false })
		h.handlingWorkflow(testkit.JSON{})

		conv, _ := h.conversation("How much is Pro per seat?")
		h.d.TC.Drain()

		s := h.load(conv, ids.UUID{})
		assert.Empty(t, s.agentReplies())
		assert.Contains(t, noteTexts(s), "path handed")
	})
}
