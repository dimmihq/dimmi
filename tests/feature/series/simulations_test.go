package series_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
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
				{"id": "wait", "kind": "wait", "settings": testkit.JSON{
					"mode": "duration", "amount": 2, "unit": "days",
				}},
				{"id": "check", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{{
					"id": "opened", "label": "Opened",
					"engagement": testkit.JSON{"node": "welcome", "action": "opened"},
				}}}},
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

	t.Run("answers extract and action steps with mocks, and later rules read tags and values", func(t *testing.T) {
		t.Parallel()
		d := newDesk(t)
		api := httpclienttest.New(d.tc.App.HTTP).PreventStrayRequests()
		vip := d.owner.PostJSON("/api/v1/tags", testkit.JSON{"name": "vip"}).AssertCreated().String("data.id")
		conn := d.owner.PostJSON("/api/v1/ai/action-connections", testkit.JSON{
			"name": "CRM", "kind": "http", "base_url": "https://crm.example.com", "auth_type": "bearer",
			"token": "sealed-token-9012",
		}).AssertCreated().String("data.id")
		action := d.owner.PostJSON("/api/v1/ai/actions", testkit.JSON{
			"connection_id": conn, "name": "lookup_seats", "title": "Look up seats", "method": "GET",
			"description": "Look up seats by email.", "path_template": "/v1/plans",
			"response_fields": []testkit.JSON{{"name": "seats", "path": "data.seats"}},
		}).AssertCreated().String("data.id")
		rule := func(field, op string, value any) testkit.JSON {
			return testkit.JSON{"rules": testkit.JSON{"field": field, "op": op, "value": value}}
		}
		id := d.series("Sizing", "contact", testkit.JSON{
			"entry": testkit.JSON{"rules": proRules(), "reentry": testkit.JSON{"kind": "once"}},
			"nodes": []testkit.JSON{
				{"id": "entry", "kind": "entry"},
				{"id": "tag", "kind": "tag", "settings": testkit.JSON{"tag_ids": []string{vip}}},
				{"id": "fit", "kind": "extract", "settings": testkit.JSON{
					"fields": []testkit.JSON{{"key": "fit", "type": "choice", "options": []string{"solo", "team"}}},
				}},
				{"id": "seats", "kind": "run_action", "settings": testkit.JSON{
					"action_id": action, "save": []testkit.JSON{{"output": "seats", "attribute": "seats"}},
				}},
				{"id": "tagged", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{
					{"id": "vip", "label": "VIP", "rules": rule("contact.tags", "has", vip)},
				}}},
				{"id": "sized", "kind": "condition", "settings": testkit.JSON{"paths": []testkit.JSON{
					{"id": "team", "label": "Team", "rules": rule("contact.custom.fit", "eq", "team")},
				}}},
				email("welcome", "Welcome, team"),
				email("sorry", "We could not look you up"),
			},
			"edges": []testkit.JSON{
				edge("entry", "", "tag"), edge("tag", "", "fit"), edge("fit", "", "seats"),
				edge("seats", "", "tagged"), edge("seats", "failed", "sorry"),
				edge("tagged", "vip", "sized"), edge("sized", "team", "welcome"),
			},
		})
		path := "/api/v1/series/" + id + "/simulations/run"
		pro := testkit.JSON{"name": "Ada", "email": "ada@example.com", "attributes": testkit.JSON{"plan": "pro"}}

		res := d.owner.PostJSON(path, testkit.JSON{"person": pro, "action_mocks": testkit.JSON{
			"fit":   testkit.JSON{"body": testkit.JSON{"fit": "team"}},
			"seats": testkit.JSON{"status": 200, "body": testkit.JSON{"seats": 12}},
		}}).AssertOK()
		res.AssertJSONPath("data.steps.2.result", "extract:ok")
		res.AssertJSONPath("data.steps.2.detail.values.fit", "team")
		res.AssertJSONPath("data.steps.3.result", "action:ok")
		res.AssertJSONPath("data.steps.3.detail.values.seats", float64(12))
		res.AssertJSONPath("data.steps.4.path", "vip")
		res.AssertJSONPath("data.steps.5.path", "team")
		res.AssertJSONPath("data.messages.0.subject", "Welcome, team")

		failed := d.owner.PostJSON(path, testkit.JSON{"person": pro, "action_mocks": testkit.JSON{
			action: testkit.JSON{"status": 500},
		}}).AssertOK()
		failed.AssertJSONPath("data.steps.3.path", "failed")
		failed.AssertJSONPath("data.messages.0.subject", "We could not look you up")
		assert.Empty(t, api.Sent(), "a dry simulation never runs the action")
	})
}
