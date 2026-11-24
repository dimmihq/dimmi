package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestAiAgentSimulationHistory(t *testing.T) {
	t.Parallel()

	t.Run("scores a simulation by its latest runs, and starts again when what it tests changes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		made := h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
			"name":       "Quotes the price",
			"messages":   []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
		}).AssertCreated()
		made.AssertJSONPath("data.draft_history", nil)
		id := made.String("data.id")
		one := testkit.JSON{"simulation_ids": []string{id}}

		h.llm.answer.Script(
			reply("Growth is $29 a seat each month."), reply("It depends on your plan."),
			reply("Growth is $29 a seat."), reply("Growth is $29."),
		)
		h.d.Owner.PostJSON(path+"/simulations/run", one).AssertOK().
			AssertJSONPath("data.0.draft_history.state", "passing").
			AssertJSONPath("data.0.draft_history.passed", float64(1)).
			AssertJSONPath("data.0.draft_history.total", float64(1)).
			AssertJSONPath("data.0.draft_history.window", float64(5))
		ran := h.d.Owner.PostJSON(path+"/simulations/run", one).AssertOK()
		ran.AssertJSONPath("data.0.draft_result.passed", false)
		ran.AssertJSONPath("data.0.draft_history.state", "flaky")
		ran.AssertJSONPath("data.0.draft_history.passed", float64(1))
		ran.AssertJSONPath("data.0.draft_history.total", float64(2))
		ran.AssertJSONPath("data.0.draft_history.runs.0.passed", false)
		ran.AssertJSONPath("data.0.draft_history.runs.1.passed", true)
		ran.AssertJSONPath("data.0.live_history", nil)

		runs := h.d.Owner.GetJSON(path + "/simulations/" + id + "/runs").AssertOK()
		runs.AssertJSONPath("data.#", 2)
		runs.AssertJSONPath("data.0.object", "ai_agent_simulation_run")
		runs.AssertJSONPath("data.0.passed", false)
		runs.AssertJSONPath("data.0.counted", true)
		runs.AssertJSONPath("data.0.result.turns.0.reply", "It depends on your plan.")
		assert.Equal(t, ran.String("data.0.draft_history.runs.0.id"), runs.String("data.0.id"))

		h.d.Owner.PatchJSON(path+"/simulations/"+id, testkit.JSON{
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "Growth"}},
		}).AssertOK().AssertJSONPath("data.draft_history", nil)
		h.d.Owner.GetJSON(path+"/simulations").AssertOK().AssertJSONPath("data.0.draft_history", nil)
		h.d.Owner.GetJSON(path+"/simulations/"+id+"/runs").AssertOK().
			AssertJSONPath("data.#", 2).AssertJSONPath("data.0.counted", false)

		h.d.Owner.PostJSON(path+"/simulations/run", one).AssertOK().
			AssertJSONPath("data.0.draft_history.state", "passing").
			AssertJSONPath("data.0.draft_history.total", float64(1))

		h.d.Owner.PatchJSON(path, testkit.JSON{"guidance": "Always greet the customer by name."}).AssertOK()
		h.d.Owner.GetJSON(path+"/simulations/"+id).AssertOK().
			AssertJSONPath("data.draft_history", nil)
		h.d.Owner.PostJSON(path+"/simulations/run", one).AssertOK().
			AssertJSONPath("data.0.draft_history.total", float64(1))
		h.d.Owner.GetJSON(path+"/simulations/"+id+"/runs").AssertOK().
			AssertJSONPath("data.#", 4).AssertJSONPath("data.0.counted", true).AssertJSONPath("data.1.counted", false)
	})

	t.Run("tells the teammate a simulation's verdict as soon as it is judged", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		out := &published{}
		h.d.TC.App.AI.Publisher = out
		path := agentPath(t, h.d)
		for _, name := range []string{"First", "Second"} {
			h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
				"name": name, "messages": []testkit.JSON{{"text": "How much is it?"}},
				"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
			}).AssertCreated()
		}
		h.llm.answer.Script(reply("Growth is $29."), reply("Growth is $29."))

		h.d.Owner.PostJSON(path+"/simulations/run", testkit.JSON{"stream_id": "stream-1"}).AssertOK()

		one := []string{"started", "turn_started", "turn_finished", "judging", "finished"}
		assert.Equal(t, append(append([]string{}, one...), one...), out.moments(),
			"the first simulation is finished before the second starts")
	})

	t.Run("looks at the last five runs only", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil)
		path := agentPath(t, h.d)
		id := h.d.Owner.PostJSON(path+"/simulations", testkit.JSON{
			"name":       "Quotes the price",
			"messages":   []testkit.JSON{{"text": "How much is it?"}},
			"assertions": []testkit.JSON{{"type": "reply_contains", "text": "$29"}},
		}).AssertCreated().String("data.id")
		h.llm.answer.Script(
			reply("No idea."), reply("$29."), reply("$29."), reply("$29."), reply("$29."), reply("$29."),
		)

		var last *testkit.Response
		for range 6 {
			last = h.d.Owner.PostJSON(path+"/simulations/run", testkit.JSON{"simulation_ids": []string{id}}).AssertOK()
		}

		last.AssertJSONPath("data.0.draft_history.state", "passing")
		last.AssertJSONPath("data.0.draft_history.total", float64(5))
		last.AssertJSONPath("data.0.draft_history.runs.#", 5)
		h.d.Owner.GetJSON(path+"/simulations/"+id+"/runs").AssertOK().AssertJSONPath("data.#", 6)
	})
}
