package series_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestSeriesDrySimulations(t *testing.T) {
	t.Parallel()

	t.Run("walks a made-up person through every step without sending or waiting", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		id := d.series("Onboarding", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				email("welcome", "Welcome to Acme"),
				{"id": "wait", "kind": "wait", "settings": testkit.JSON{"mode": "duration", "amount": 2, "unit": "days"}},
				{"id": "check", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{
					{"id": "opened", "label": "Opened", "engagement": testkit.JSON{"node": "welcome", "action": "opened"}},
				}}},
				email("nudge", "Did you see this?"),
			},
			"edges": []testkit.JSON{
				edge("entry", "", "welcome"), edge("welcome", "", "wait"), edge("wait", "", "check"),
				edge("check", "else", "nudge"),
			},
		})
		path := "/api/v1/series/" + id + "/simulations/run"
		pro := testkit.JSON{"name": "Ada", "email": "ada@example.com", "attributes": testkit.JSON{"plan": "pro"}}

		res := d.owner.PostJSON(path, testkit.JSON{"person": pro}).AssertOK()
		res.AssertJSONPath("data.object", "series_simulation")
		res.AssertJSONPath("data.entered", true)
		res.AssertJSONPath("data.steps.2.node_id", "wait")
		res.AssertJSONPath("data.steps.2.skipped", true)
		res.AssertJSONPath("data.steps.3.path", "else")
		res.AssertJSONPath("data.messages.#", 2)
		res.AssertJSONPath("data.messages.1.subject", "Did you see this?")
		res.AssertJSONPath("data.end", "finished")
		assert.Equal(t, 0, mailCount(t, "ada@example.com"), "a dry simulation sends nothing")

		opened := d.owner.PostJSON(path, testkit.JSON{"person": pro, "engaged": []string{"opened"}}).AssertOK()
		opened.AssertJSONPath("data.steps.3.path", "opened")
		opened.AssertJSONPath("data.messages.#", 1)

		free := testkit.JSON{"name": "Bo", "attributes": testkit.JSON{"plan": "free"}}
		d.owner.PostJSON(path, testkit.JSON{"person": free}).AssertOK().
			AssertJSONPath("data.entered", false).AssertJSONPath("data.reason", "entry_rules")
		d.owner.PostJSON(path, testkit.JSON{"version": "live", "person": pro}).AssertValidationError("version")
		d.owner.PostJSON(path, testkit.JSON{}).AssertValidationError("contact_id")
	})
}
