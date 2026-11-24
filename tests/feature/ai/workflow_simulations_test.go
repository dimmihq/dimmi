package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

func TestWorkflowSimulationAgentTurns(t *testing.T) {
	t.Parallel()

	t.Run("plays dimmi's turns inside an agent step and takes the path of how they ended", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		h.llm.answer.Script(reply("Growth is $29 a seat each month."))
		h.player.moves = []pipeline.CustomerMove{{Done: true}}
		wf := h.d.Owner.PostJSON("/api/v1/workflows", testkit.JSON{"name": "Agent first", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{{
				"id": "s_dimmi01", "kind": "dimmi", "settings": testkit.JSON{},
				"paths": []testkit.JSON{
					{"id": "p_solved1", "outcome": "resolved", "steps": []testkit.JSON{
						{"id": "s_glad001", "kind": "message", "settings": testkit.JSON{"text": "Glad that helped."}},
					}},
				},
			}},
		}}).AssertCreated().String("data.id")
		sims := "/api/v1/workflows/" + wf + "/simulations"
		h.d.Owner.PostJSON(sims, testkit.JSON{
			"name": "Quotes the price", "messages": []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{
				{"type": "reply_contains", "text": "$29"},
				{"type": "path_taken", "step_id": "s_dimmi01", "branch": "p_solved1"},
			},
		}).AssertCreated()

		ran := h.d.Owner.PostJSON(sims+"/run", nil).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", true)
		ran.AssertJSONPath("data.0.draft_result.turns.0.message", "How much is it?")
		ran.AssertJSONPath("data.0.draft_result.turns.0.reply", "Growth is $29 a seat each month.")
		ran.AssertJSONPath("data.0.draft_result.transcript.1.from", "agent")
		ran.AssertJSONPath("data.0.draft_result.transcript.2.text", "Glad that helped.")
		assert.Contains(t, ran.String("data.0.draft_result.turns.0.run_id"), "air_")
	})
}
