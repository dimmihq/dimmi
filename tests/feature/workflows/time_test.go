package workflows_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
note is a step that leaves an internal note.
*/
func note(id, text string) testkit.JSON {
	return step(id, "note", testkit.JSON{"text": text})
}

/*
saturday moves the clock from the Monday morning tests start on to the
Saturday after, when the default schedule is closed.
*/
func saturday(d tests.Desk) {
	d.TC.Clock.Travel(5 * 24 * time.Hour)
}

func TestSchedules(t *testing.T) {
	t.Parallel()

	t.Run("gives every workspace a default schedule open on weekdays", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		res := d.Owner.GetJSON("/api/v1/schedules").AssertOK()
		res.AssertJSONPath("data.#", float64(1)).AssertJSONPath("data.0.name", "Office hours").
			AssertJSONPath("data.0.is_default", true).AssertJSONPath("data.0.status.open", true)
		assert.Equal(t, "09:00", res.String("data.0.weekly.mon.0.0"))
	})

	t.Run("adds a schedule, makes it the default and keeps exactly one default", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		d.Owner.GetJSON("/api/v1/schedules").AssertOK()

		id := d.Owner.PostJSON("/api/v1/schedules", testkit.JSON{
			"name": "Weekend", "timezone": "Europe/Berlin",
			"weekly":   testkit.JSON{"sat": [][]string{{"10:00", "14:00"}}},
			"holidays": []testkit.JSON{{"date": "2026-12-25", "name": "Christmas"}},
		}).AssertCreated().String("data.id")
		d.Owner.PatchJSON("/api/v1/schedules/"+id, testkit.JSON{"is_default": true}).AssertOK().
			AssertJSONPath("data.is_default", true)

		d.Owner.GetJSON("/api/v1/schedules").AssertOK().AssertJSONPath("data.0.id", id).
			AssertJSONPath("data.1.is_default", false)
	})

	t.Run("refuses hours that cannot be read", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/schedules", testkit.JSON{
			"name": "Broken", "timezone": "UTC", "weekly": testkit.JSON{"mon": [][]string{{"17:00", "09:00"}}},
		}).AssertStatus(422).AssertJSONPath("code", "schedule_invalid")
		d.Owner.PostJSON("/api/v1/schedules", testkit.JSON{
			"name": "Nowhere", "timezone": "Mars/Olympus", "weekly": testkit.JSON{},
		}).AssertStatus(422)
	})

	t.Run("refuses to delete the default or a schedule a workflow uses", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		def := d.Owner.GetJSON("/api/v1/schedules").AssertOK().String("data.0.id")
		other := d.Owner.PostJSON("/api/v1/schedules", testkit.JSON{
			"name": "Nights", "timezone": "UTC", "weekly": testkit.JSON{"mon": [][]string{{"20:00", "24:00"}}},
		}).AssertCreated().String("data.id")
		create(t, d, testkit.JSON{"name": "At night", "spec": testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"schedule": testkit.JSON{"mode": "open", "schedule_id": other}, "steps": []testkit.JSON{},
		}})

		d.Owner.Delete("/api/v1/schedules/"+def).AssertStatus(409).AssertJSONPath("code", "schedule_default")
		d.Owner.Delete("/api/v1/schedules/"+other).AssertStatus(409).AssertJSONPath("code", "schedule_in_use")
	})
}

func TestOfficeHours(t *testing.T) {
	t.Parallel()

	t.Run("runs a workflow kept to office hours only while the team is in", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Hours only", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message", "schedule": testkit.JSON{"mode": "open"},
			"steps": []testkit.JSON{note("s_note0001", "in hours")},
		})

		assert.Contains(t, bodies(t, d, conversation(t, d, "ana", "Hi")), "in hours")
		saturday(d)
		assert.NotContains(t, bodies(t, d, conversation(t, d, "bo", "Hi")), "in hours")
	})

	t.Run("branches on whether the team is in", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "After hours", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{step("s_branch01", "branch", testkit.JSON{},
				testkit.JSON{
					"id": "p_closed01", "label": "Closed", "steps": []testkit.JSON{note("s_note0001", "closed")},
					"condition": testkit.JSON{"field": "office_hours.open", "op": "is_false"},
				},
				testkit.JSON{
					"id": "p_else0001", "outcome": "else", "steps": []testkit.JSON{note("s_note0002", "open")},
				})},
		})

		assert.Contains(t, bodies(t, d, conversation(t, d, "ana", "Hi")), "open")
		saturday(d)
		assert.Contains(t, bodies(t, d, conversation(t, d, "bo", "Hi")), "closed")
	})

	t.Run("waits until the team is back before going on", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Wait for Monday", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_wait0001", "wait", testkit.JSON{
					"until":        testkit.JSON{"kind": "office_hours_open"},
					"interrupt_on": []string{"teammate_reply"},
				}),
				note("s_note0001", "back in"),
			},
		})
		saturday(d)

		conv := conversation(t, d, "cy", "Hi")
		assert.NotContains(t, bodies(t, d, conv), "back in")

		d.TC.Clock.Travel(24 * time.Hour)
		d.TC.DrainLater()
		assert.NotContains(t, bodies(t, d, conv), "back in", "Sunday is still closed")

		d.TC.Clock.Travel(24 * time.Hour)
		d.TC.DrainLater()
		assert.Contains(t, bodies(t, d, conv), "back in")
	})

	t.Run("counts only open hours in a business wait", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Two business hours", testkit.JSON{
			"schema": 1, "trigger": "conversation.first_message",
			"steps": []testkit.JSON{
				step("s_wait0001", "wait", testkit.JSON{
					"for": "2h", "count": "business",
					"interrupt_on": []string{"teammate_reply"},
				}),
				note("s_note0001", "two hours on"),
			},
		})
		d.TC.Clock.Travel(7 * time.Hour)

		conv := conversation(t, d, "di", "Hi")
		d.TC.Clock.Travel(3 * time.Hour)
		d.TC.DrainLater()
		assert.NotContains(t, bodies(t, d, conv), "two hours on", "the evening does not count")

		d.TC.Clock.Travel(14 * time.Hour)
		d.TC.DrainLater()
		assert.NotContains(t, bodies(t, d, conv), "two hours on", "one open hour on Tuesday is not two")

		d.TC.Clock.Travel(time.Hour)
		d.TC.DrainLater()
		assert.Contains(t, bodies(t, d, conv), "two hours on")
	})
}

func TestQuietTriggers(t *testing.T) {
	t.Parallel()

	quiet := func(t *testing.T, d tests.Desk) {
		t.Helper()
		live(t, d, "Nudge", testkit.JSON{
			"schema": 1, "trigger": "conversation.customer_quiet", "trigger_settings": testkit.JSON{"after": "1h"},
			"steps": []testkit.JSON{note("s_note0001", "customer went quiet")},
		})
	}

	t.Run("fires once the customer says nothing after a teammate's reply", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		quiet(t, d)
		conv := conversation(t, d, "ana", "Hi")
		d.Owner.PostJSON(conv+"/replies", testkit.JSON{"body": "Here is how."}).AssertCreated()
		d.TC.Drain()

		d.TC.Clock.Travel(61 * time.Minute)
		d.TC.DrainLater()
		assert.Contains(t, bodies(t, d, conv), "customer went quiet")
	})

	t.Run("stays quiet when the customer writes back in time", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		quiet(t, d)
		conv := conversation(t, d, "bo", "Hi")
		d.Owner.PostJSON(conv+"/replies", testkit.JSON{"body": "Here is how."}).AssertCreated()
		d.TC.Drain()
		d.TC.Clock.Travel(30 * time.Minute)
		d.Owner.PostJSON(conv+"/messages", testkit.JSON{"body": "Thanks, trying now"}).AssertCreated()
		d.TC.Drain()

		d.TC.Clock.Travel(31 * time.Minute)
		d.TC.DrainLater()
		assert.NotContains(t, bodies(t, d, conv), "customer went quiet")
	})

	t.Run("counts the silence from the newest reply", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		quiet(t, d)
		conv := conversation(t, d, "cy", "Hi")
		d.Owner.PostJSON(conv+"/replies", testkit.JSON{"body": "First answer."}).AssertCreated()
		d.TC.Drain()
		d.TC.Clock.Travel(50 * time.Minute)
		d.Owner.PostJSON(conv+"/replies", testkit.JSON{"body": "One more thing."}).AssertCreated()
		d.TC.Drain()

		d.TC.Clock.Travel(11 * time.Minute)
		d.TC.DrainLater()
		notes := 0
		for _, b := range bodies(t, d, conv) {
			if b == "customer went quiet" {
				notes++
			}
		}

		require.Equal(t, 0, notes, "the first reply's timer must not fire")
		d.TC.Clock.Travel(time.Hour)
		d.TC.DrainLater()
		assert.Contains(t, bodies(t, d, conv), "customer went quiet")
	})

	t.Run("fires when the customer waits for the team too long, once per wait", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		live(t, d, "Escalate", testkit.JSON{
			"schema": 1, "trigger": "conversation.teammate_quiet", "trigger_settings": testkit.JSON{"after": "30m"},
			"steps": []testkit.JSON{note("s_note0001", "still waiting")},
		})

		answered := conversation(t, d, "di", "Anyone there?")
		d.TC.Clock.Travel(29 * time.Minute)
		d.Owner.PostJSON(answered+"/replies", testkit.JSON{"body": "Yes, here."}).AssertCreated()
		d.TC.Drain()

		waiting := conversation(t, d, "ed", "Hello?")
		d.TC.Clock.Travel(31 * time.Minute)
		d.TC.DrainLater()

		assert.NotContains(t, bodies(t, d, answered), "still waiting")
		assert.Contains(t, bodies(t, d, waiting), "still waiting")
	})
}
