package series_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
)

func TestSeriesSimulationScenario(t *testing.T) {
	t.Parallel()

	t.Run("asks the model what the scenario's person does, and keeps each answer", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		withModel(t, d, fake.New(contracts.LLMProviderAnthropic).Script(
			fake.Step{Text: `{"choice":"yes","confidence":"high"}`},
		))
		id := d.series("Trial", "contact", testkit.JSON{
			"entry": testkit.JSON{"event": "trial_started", "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				email("welcome", "Welcome to Acme"),
				{"id": "check", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{{
					"id": "opened", "label": "Opened",
					"engagement": testkit.JSON{"node": "welcome", "action": "opened"},
				}}}},
				email("nudge", "Did you see this?"),
			},
			"edges": []testkit.JSON{
				edge("entry", "", "welcome"), edge("welcome", "", "check"), edge("check", "else", "nudge"),
			},
		})

		res := d.owner.PostJSON("/api/v1/series/"+id+"/simulations/run", testkit.JSON{
			"person":   testkit.JSON{"name": "Ada", "email": "ada@example.com"},
			"scenario": "Opens every email the day it lands.",
		}).AssertOK()

		res.AssertJSONPath("data.entered", true)
		res.AssertJSONPath("data.steps.2.path", "opened")
		res.AssertJSONPath("data.messages.#", 1)
		res.AssertJSONPath("data.decisions.#", 1)
		res.AssertJSONPath("data.decisions.0.before", 2)
		res.AssertJSONPath("data.decisions.0.did", true)
		assert.Contains(t, res.String("data.decisions.0.what"), `"Welcome to Acme"`)
	})
}
