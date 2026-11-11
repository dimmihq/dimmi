package ai_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
openWeek is a schedule open around the clock every day.
*/
func openWeek() testkit.JSON {
	day := [][]string{{"00:00", "24:00"}}

	return testkit.JSON{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day, "sat": day, "sun": day}
}

/*
closedWeek is a schedule closed every day.
*/
func closedWeek() testkit.JSON {
	return testkit.JSON{}
}

/*
defaultHours sets the workspace's default office hours to the week.
*/
func (h *harness) defaultHours(weekly testkit.JSON) {
	h.t.Helper()
	id := h.d.Owner.GetJSON("/api/v1/schedules").AssertOK().String("data.0.id")
	h.d.Owner.PatchJSON("/api/v1/schedules/"+id, testkit.JSON{"timezone": "UTC", "weekly": weekly}).AssertOK()
}

/*
inboxHours gives the desk's inbox office hours of its own, set to the
week.
*/
func (h *harness) inboxHours(weekly testkit.JSON) {
	h.t.Helper()
	id := h.d.Owner.PatchJSON("/api/v1/inboxes/"+h.d.InboxID()+"/hours", testkit.JSON{"own": true}).
		AssertOK().String("data.id")
	h.d.Owner.PatchJSON("/api/v1/schedules/"+id, testkit.JSON{"timezone": "UTC", "weekly": weekly}).AssertOK()
}

func TestAgentOfficeHours(t *testing.T) {
	t.Parallel()

	outside := func(p *models.UpdateAgentParams) { p.Schedule = "outside_office_hours" }
	inside := func(p *models.UpdateAgentParams) { p.Schedule = "inside_office_hours" }

	t.Run("answers outside the inbox's own hours while the default is open", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, outside)
		h.defaultHours(openWeek())
		h.inboxHours(closedWeek())

		conv, trigger := h.conversation("How much is Pro?")
		assert.Empty(t, h.run(conv, trigger).SkipReason)
	})

	t.Run("holds back inside the inbox's own hours while the default is closed", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, outside)
		h.defaultHours(closedWeek())
		h.inboxHours(openWeek())

		conv, trigger := h.conversation("How much is Pro?")
		assert.Equal(t, "schedule", h.run(conv, trigger).SkipReason)
	})

	t.Run("follows the workspace default when the inbox has no hours of its own", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, inside)
		h.defaultHours(closedWeek())

		conv, trigger := h.conversation("How much is Pro?")
		assert.Equal(t, "schedule", h.run(conv, trigger).SkipReason)

		h.defaultHours(openWeek())
		conv, trigger = h.conversation("And Team?")
		assert.Empty(t, h.run(conv, trigger).SkipReason)
	})

	t.Run("takes the default back once the inbox gives up its own hours", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, inside)
		h.defaultHours(openWeek())
		h.inboxHours(closedWeek())
		h.d.Owner.PatchJSON("/api/v1/inboxes/"+h.d.InboxID()+"/hours", testkit.JSON{"own": false}).AssertOK().
			AssertJSONPath("data.is_default", true)

		conv, trigger := h.conversation("How much is Pro?")
		assert.Empty(t, h.run(conv, trigger).SkipReason)
	})
}
