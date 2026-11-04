package assignment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

/*
inboxPath returns the API path of the desk's default inbox.
*/
func (d desk) inboxPath() string {
	return "/api/v1/inboxes/" + models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)
}

/*
extraInbox adds an inbox through the API and returns its public id.
*/
func (d desk) extraInbox(t *testing.T, name string) string {
	t.Helper()

	return d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": name}).AssertCreated().String("data.id")
}

func TestInboxSettings(t *testing.T) {
	t.Parallel()

	t.Run("shows the defaults and marks the default inbox", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := d.extraInbox(t, "Billing")

		list := d.owner.GetJSON("/api/v1/inboxes").AssertOK()
		list.AssertJSONPath("data.0.default", true)
		list.AssertJSONPath("data.1.id", other)
		list.AssertJSONPath("data.1.default", false)
		list.AssertJSONPath("data.1.settings.reopen_window_days", 30)
		list.AssertJSONPath("data.1.settings.assign_on_reply", true)
		list.AssertJSONPath("data.1.settings.assignment.mode", "manual")
		list.AssertJSONPath("data.1.settings.assignment.queue_order", "oldest_waiting")
		list.AssertJSONPath("data.1.default_team_id", nil)
	})

	t.Run("patches settings and keeps what the patch leaves out", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.team(t, "Support")
		overflow := d.team(t, "Overflow")

		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{
			"name": "Help", "default_team_id": team,
			"settings": testkit.JSON{
				"reopen_window_days": 7, "color": "#3b82f6",
				"assignment": testkit.JSON{"mode": "round_robin", "overflow_team_id": overflow, "default_capacity": 5},
			},
		}).AssertOK().
			AssertJSONPath("data.name", "Help").
			AssertJSONPath("data.default_team_id", team).
			AssertJSONPath("data.settings.assignment.overflow_team_id", overflow)

		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{
			"settings": testkit.JSON{"assignment": testkit.JSON{"queue_order": "priority_then_oldest"}},
		}).AssertOK().
			AssertJSONPath("data.settings.reopen_window_days", 7).
			AssertJSONPath("data.settings.color", "#3b82f6").
			AssertJSONPath("data.settings.assignment.mode", "round_robin").
			AssertJSONPath("data.settings.assignment.default_capacity", 5).
			AssertJSONPath("data.settings.assignment.queue_order", "priority_then_oldest")

		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{
			"default_team_id": nil, "settings": testkit.JSON{"assignment": testkit.JSON{"overflow_team_id": nil}},
		}).AssertOK().
			AssertJSONPath("data.default_team_id", nil).
			AssertJSONPath("data.settings.assignment.overflow_team_id", nil)
	})

	t.Run("refuses settings outside what they accept", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.team(t, "Support")

		for _, body := range []testkit.JSON{
			{"settings": testkit.JSON{"reopen_window_days": 366}},
			{"settings": testkit.JSON{"color": "blue"}},
			{"settings": testkit.JSON{"assignment": testkit.JSON{"mode": "lottery"}}},
			{"settings": testkit.JSON{"assignment": testkit.JSON{"queue_order": "newest"}}},
			{"settings": testkit.JSON{"assignment": testkit.JSON{"default_capacity": 0}}},
			{"settings": testkit.JSON{"assignment": testkit.JSON{"capacity_by_channel": testkit.JSON{"fax": 2}}}},
			{"settings": "everything"},
		} {
			d.owner.PatchJSON(d.inboxPath(), body).AssertStatus(422)
		}

		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{"default_team_id": "team_nope"}).
			AssertStatus(422).AssertJSONPath("code", "team_not_found")
		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{
			"settings": testkit.JSON{"assignment": testkit.JSON{"overflow_team_id": "team_nope"}},
		}).AssertStatus(422).AssertJSONPath("code", "team_not_found")
		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{
			"default_team_id": team, "settings": testkit.JSON{"assignment": testkit.JSON{"overflow_team_id": team}},
		}).AssertStatus(422).AssertJSONPath("code", "overflow_team_is_default")

		d.owner.GetJSON(d.inboxPath()).AssertOK().AssertJSONPath("data.default_team_id", nil)
	})

	t.Run("lets only those who manage inboxes change them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)

		m.client.GetJSON(d.inboxPath()).AssertOK()
		m.client.PatchJSON(d.inboxPath(), testkit.JSON{"name": "Mine"}).AssertForbidden()
		m.client.Delete(d.inboxPath()).AssertForbidden()
	})

	t.Run("changes nothing for a patch that changes nothing", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		before := d.owner.GetJSON(d.inboxPath()).AssertOK().String("data.updated_at")

		d.owner.PatchJSON(d.inboxPath(), testkit.JSON{"settings": testkit.JSON{"reopen_window_days": 30}}).
			AssertOK().AssertJSONPath("data.updated_at", before)
	})
}

func TestDeleteInbox(t *testing.T) {
	t.Parallel()

	t.Run("never deletes the default inbox", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.Delete(d.inboxPath()).AssertStatus(422).AssertJSONPath("code", "default_inbox")
	})

	t.Run("deletes an empty inbox without a target", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := d.extraInbox(t, "Billing")

		d.owner.Delete("/api/v1/inboxes/" + other).AssertNoContent()
		d.owner.GetJSON("/api/v1/inboxes/" + other).AssertNotFound()
	})

	t.Run("moves conversations to the target and refuses a bad one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := d.extraInbox(t, "Billing")
		conversation := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": other, "author": "contact", "subject": "Refund", "body": "Please",
			"contact": testkit.JSON{"email": "bo@example.com", "name": "Bo"},
		}).AssertCreated().String("data.id")
		path := "/api/v1/inboxes/" + other

		d.owner.Delete(path).AssertStatus(422).AssertJSONPath("code", "move_to_required")
		d.owner.Delete(path+"?move_to="+other).AssertStatus(422).AssertJSONPath("code", "move_to_invalid")
		d.owner.Delete(path+"?move_to=inbox_nope").AssertStatus(422).AssertJSONPath("code", "move_to_invalid")

		target := models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)
		d.owner.Delete(path + "?move_to=" + target).AssertNoContent()

		d.show(t, conversation).AssertJSONPath("data.inbox.id", target)
		list := d.owner.GetJSON("/api/v1/inboxes").AssertOK()
		assert.NotContains(t, string(list.Body), other)
	})
}
