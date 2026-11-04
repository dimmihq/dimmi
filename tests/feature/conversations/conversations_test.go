package conversations_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestConversationsStore(t *testing.T) {
	t.Parallel()

	t.Run("starts a customer conversation nobody holds, numbered from one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		created := record[appevents.ConversationCreated](d.tc)

		res := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "subject": "Export failing", "body": "It spins forever.",
			"contact": testkit.JSON{"external_id": "user_42", "email": "Bob@Acme.dev", "name": "Bob Lee"},
		}).AssertCreated().
			AssertJSONPath("data.object", "conversation").
			AssertJSONPath("data.number", 1).
			AssertJSONPath("data.status", "open").
			AssertJSONPath("data.assignee", nil).
			AssertJSONPath("data.channel.type", "api").
			AssertJSONPath("data.inbox.name", "Support").
			AssertJSONPath("data.contact.email", "bob@acme.dev").
			AssertJSONPath("data.contact.kind", "user").
			AssertJSONPath("data.last_message.excerpt", "It spins forever.").
			AssertJSONPath("data.last_message.author.name", "Bob Lee").
			AssertJSONPath("data.unread", true)

		d.owner.GetJSON("/api/v1/conversations/1").AssertOK().AssertJSONPath("data.id", res.String("data.id"))
		require.Len(t, created.all(), 1)
		assert.Equal(t, "contact", created.all()[0].InitiatedBy)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Again",
			"contact": testkit.JSON{"external_id": "user_42"},
		}).AssertCreated().
			AssertJSONPath("data.number", 2).
			AssertJSONPath("data.contact.id", res.String("data.contact.id"))
	})

	t.Run("makes a teammate's conversation theirs unless told otherwise", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		channel := models.PublicID(models.PrefixChannelConnection, d.inbox.API.ID)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": channel, "body": "Following up", "contact": testkit.JSON{"email": "ann@x.dev"},
		}).AssertCreated().
			AssertJSONPath("data.assignee.id", d.ownerID()).
			AssertJSONPath("data.unread", false).
			AssertJSONPath("data.last_message.author.type", "member")

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": channel, "body": "Following up", "contact": testkit.JSON{"email": "ann@x.dev"},
			"assignee_id": nil,
		}).AssertCreated().AssertJSONPath("data.assignee", nil)
	})

	t.Run("refuses a conversation without a customer, an inbox or a body", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "body": "Hi", "contact": testkit.JSON{},
		}).AssertStatus(422).AssertJSONPath("code", "contact_required")
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": "inbox_nope", "author": "contact", "body": "Hi", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422).AssertJSONPath("code", "inbox_not_found")
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"author": "contact", "body": "Hi", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422).AssertJSONPath("code", "inbox_not_found")
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.inboxID(), "author": "contact", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422)
	})

	t.Run("refuses customer messages off the api channel and channels that cannot initiate", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		widget, err := factories.Connection(t.Context(), d.tc.App, d.inbox.Inbox, "widget")
		require.NoError(t, err)
		id := models.PublicID(models.PrefixChannelConnection, widget.ID)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": id, "author": "contact", "body": "Hi", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(409).AssertJSONPath("code", "channel_not_api")
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": id, "body": "Hi", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422).AssertJSONPath("code", "channel_cannot_initiate")
		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": "chn_nope", "body": "Hi", "contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422).AssertJSONPath("code", "channel_cannot_initiate")
	})

	t.Run("refuses an inbox without an api channel", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		res := d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertCreated()
		_, err := d.tc.Operator().Exec(t.Context(),
			"DELETE FROM channel_connections WHERE type = 'api' AND inbox_id <> $1",
			d.inbox.Inbox.ID)
		require.NoError(t, err)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": res.String("data.id"), "author": "contact", "body": "Hi",
			"contact": testkit.JSON{"email": "a@b.co"},
		}).AssertStatus(422).AssertJSONPath("code", "inbox_has_no_api_channel")
	})
}

func TestConversationsShow(t *testing.T) {
	t.Parallel()

	t.Run("shows by id and number, and nothing of another workspace", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "bob", "Refund", "Please refund me")
		other := open(t)

		d.owner.GetJSON("/api/v1/conversations/"+id).AssertOK().AssertJSONPath("data.subject", "Refund")
		d.owner.GetJSON("/api/v1/conversations/1").AssertOK().AssertJSONPath("data.id", id)
		other.owner.GetJSON("/api/v1/conversations/" + id).AssertNotFound()
		other.owner.GetJSON("/api/v1/conversations/1").AssertNotFound()
		d.owner.GetJSON("/api/v1/conversations/conv_nope").AssertNotFound()
		d.owner.GetJSON("/api/v1/conversations/ctc_01jbd0k3a8x9t2m4q5r6s7v8w9").AssertNotFound()
	})

	t.Run("hides deleted conversations", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "bob", "Refund", "Please refund me")
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE conversations SET deleted_at = now()")
		require.NoError(t, err)

		d.owner.GetJSON("/api/v1/conversations/" + id).AssertNotFound()
	})
}

func TestConversationsUpdate(t *testing.T) {
	t.Parallel()

	t.Run("assigns then closes in one call, in that order, once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, bob := d.join(t, enums.RoleMember)
		id := d.customer(t, "ann", "Billing", "Charged twice")
		all := record[any](d.tc)

		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{
			"status": "closed", "assignee_id": memberID(bob), "note": "Handing over to Bob.",
		}).AssertOK().
			AssertJSONPath("data.status", "closed").
			AssertJSONPath("data.assignee.id", memberID(bob)).
			AssertJSONPath("meta.changed", []any{"assignee_id", "status"})

		var names []string
		for _, e := range all.all() {
			switch e.(type) {
			case appevents.ConversationAssigned:
				names = append(names, "assigned")
			case appevents.ConversationClosed:
				names = append(names, "closed")
			case appevents.MessageCreated:
				names = append(names, "note")
			}
		}

		assert.Equal(t, []string{"assigned", "closed", "note"}, names)

		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{
			"status": "closed", "assignee_id": memberID(bob),
		}).AssertOK().AssertJSONPath("meta.changed", []any{})

		timeline := d.owner.GetJSON("/api/v1/conversations/" + id + "/messages").AssertOK()
		assert.Equal(t, []string{"message", "event", "event", "note"}, sliceOf(timeline, "data.#.type"))
		timeline.AssertJSONPath("data.1.event.name", "conversation.assigned").
			AssertJSONPath("data.1.event.to.member.id", memberID(bob)).
			AssertJSONPath("data.2.event.name", "conversation.closed").
			AssertJSONPath("data.3.body", "Handing over to Bob.")
	})

	t.Run("reopens, snoozes and wakes, refusing a snooze from closed", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Billing", "Charged twice")
		path := "/api/v1/conversations/" + id

		d.owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "tomorrow"}).
			AssertStatus(422).
			AssertJSONPath("code", "invalid_transition").
			AssertJSONPath("from", "closed").
			AssertJSONPath("to", "snoozed")
		d.owner.PatchJSON(path, testkit.JSON{"status": "open"}).AssertOK().AssertJSONPath("data.closed_at", nil)

		snoozed := d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "tomorrow"}).AssertOK().
			AssertJSONPath("data.status", "snoozed")
		assertTime(t, snoozed, "data.snoozed_until", "2026-11-03T09:00:00Z")
		d.owner.PatchJSON(path, testkit.JSON{"status": "open"}).AssertOK().AssertJSONPath("data.snoozed_until", nil)

		events := sliceOf(d.owner.GetJSON(path+"/messages?filter[type]=event").AssertOK(), "data.#.event.name")
		assert.Equal(t, []string{
			"conversation.closed", "conversation.reopened", "conversation.snoozed", "conversation.unsnoozed",
		}, events)
	})

	t.Run("snoozes until a time within the limits", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		now := d.tc.Clock.Now()

		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snoozed_until": now.Add(30 * time.Second)}).
			AssertStatus(422).AssertJSONPath("code", "snooze_in_past")
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snoozed_until": now.Add(400 * 24 * time.Hour)}).
			AssertStatus(422).AssertJSONPath("code", "snooze_too_far")
		later := d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snoozed_until": now.Add(2 * time.Hour)}).
			AssertOK()
		assertTime(t, later, "data.snoozed_until", "2026-11-02T11:00:00Z")
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snoozed_until": now.Add(2 * time.Hour)}).
			AssertOK().AssertJSONPath("meta.changed", []any{})
		d.owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK().AssertJSONPath("data.snoozed_until", nil)
	})

	t.Run("refuses side fields without their field and unknown fields", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")

		d.owner.PatchJSON(path, testkit.JSON{"snooze_preset": "tomorrow"}).AssertValidationError("snooze_preset")
		d.owner.PatchJSON(path, testkit.JSON{"snoozed_until": d.tc.Clock.Now()}).AssertValidationError("snoozed_until")
		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed"}).AssertValidationError("snoozed_until")
		d.owner.PatchJSON(path, testkit.JSON{"note": "x"}).AssertValidationError("note")
		d.owner.PatchJSON(path, testkit.JSON{"status": "done"}).AssertStatus(422)
		d.owner.PatchJSON(path, testkit.JSON{"trashed": true}).AssertStatus(422)
	})

	t.Run("refuses an inactive assignee, an unknown team and inbox, and rolls every field back", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, gone := d.join(t, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), d.tc.App, gone))
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")

		d.owner.PatchJSON(path, testkit.JSON{"priority": "high", "assignee_id": memberID(gone)}).
			AssertStatus(422).AssertJSONPath("code", "member_inactive")
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": "mem_nope"}).
			AssertStatus(422).AssertJSONPath("code", "member_inactive")
		d.owner.PatchJSON(path, testkit.JSON{"team_id": "team_01jbd0k3a8x9t2m4q5r6s7v8w9"}).
			AssertStatus(422).AssertJSONPath("code", "team_not_found")
		d.owner.PatchJSON(path, testkit.JSON{"team_id": "nope"}).AssertStatus(422)
		d.owner.PatchJSON(path, testkit.JSON{"inbox_id": "inbox_01jbd0k3a8x9t2m4q5r6s7v8w9"}).
			AssertStatus(422).AssertJSONPath("code", "inbox_not_found")
		d.owner.PatchJSON(path, testkit.JSON{"inbox_id": "nope"}).AssertStatus(422)

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.priority", "none")
	})

	t.Run("assigns to me and a team, and unassigns with null", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := createTeam(t, d, "Billing")
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")

		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": "me", "team_id": team}).AssertOK().
			AssertJSONPath("data.assignee.id", d.ownerID()).
			AssertJSONPath("data.team.id", team).
			AssertJSONPath("meta.changed", []any{"assignee_id", "team_id"})
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": nil}).AssertOK().
			AssertJSONPath("data.assignee", nil).
			AssertJSONPath("data.team.id", team)
		d.owner.PatchJSON(path, testkit.JSON{"team_id": nil}).AssertOK().AssertJSONPath("data.team", nil)
	})

	t.Run("changes subject, priority, inbox and spam, each said once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		sales := d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertCreated().String("data.id")
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		updated := record[appevents.ConversationUpdated](d.tc)

		d.owner.PatchJSON(path, testkit.JSON{"subject": "Double charge", "priority": "high", "inbox_id": sales}).
			AssertOK().
			AssertJSONPath("data.subject", "Double charge").
			AssertJSONPath("data.priority", "high").
			AssertJSONPath("data.inbox.id", sales).
			AssertJSONPath("meta.changed", []any{"subject", "priority", "inbox_id"})
		require.Len(t, updated.all(), 1)
		assert.Len(t, updated.all()[0].Changes, 2)

		d.owner.PatchJSON(path, testkit.JSON{"spam": true}).AssertOK().
			AssertJSONPath("data.spam", true).
			AssertJSONPath("data.status", "open")
		d.owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertOK().AssertJSONPath("data.#", 0)
		d.owner.GetJSON("/api/v1/conversations?filter[view]=spam").AssertOK().AssertJSONPath("data.#", 1)
		d.owner.PatchJSON(path, testkit.JSON{"spam": false, "priority": "none"}).AssertOK().
			AssertJSONPath("meta.changed", []any{"spam", "priority"})
	})

	t.Run("marks the conversation unread and read for the caller only", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")

		d.owner.PatchJSON(path, testkit.JSON{"unread": false}).AssertOK().
			AssertJSONPath("data.unread", false).
			AssertJSONPath("meta.changed", []any{"unread"})
		bob.GetJSON(path).AssertOK().AssertJSONPath("data.unread", true)
		d.owner.PatchJSON(path, testkit.JSON{"unread": true}).AssertOK().AssertJSONPath("data.unread", true)
		d.owner.PatchJSON(path, testkit.JSON{"unread": true}).AssertOK().AssertJSONPath("meta.changed", []any{})
	})
}

func TestConversationsPermissions(t *testing.T) {
	t.Parallel()

	t.Run("lets members work conversations and keeps tokens to their abilities", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		member, m := d.join(t, enums.RoleMember)
		id := d.customer(t, "ann", "Billing", "Charged twice")

		member.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		reader := d.tc.ActingAsToken(m, "conversations:read")
		reader.GetJSON("/api/v1/conversations/" + id).AssertOK()
		reader.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "open"}).AssertForbidden()
		reader.PostJSON("/api/v1/conversations/"+id+"/replies", testkit.JSON{"body": "x"}).AssertForbidden()

		writer := d.tc.ActingAsToken(m, "contacts:read")
		writer.GetJSON("/api/v1/conversations").AssertForbidden()
		writer.GetJSON("/api/v1/conversation-counts").AssertForbidden()
	})
}

/*
sliceOf returns the strings at the path of the response, such as
data.#.type.
*/
func sliceOf(res *testkit.Response, path string) []string {
	var out []string
	if err := json.Unmarshal([]byte(res.String(path)), &out); err != nil {
		return nil
	}

	return out
}

/*
assertTime asserts the time at the path is the instant given.
*/
func assertTime(t *testing.T, res *testkit.Response, path, want string) {
	t.Helper()

	got, err := time.Parse(time.RFC3339, res.String(path))
	require.NoError(t, err)

	expected, err := time.Parse(time.RFC3339, want)
	require.NoError(t, err)

	assert.True(t, expected.Equal(got), "%s is %s, want %s", path, got, want)
}

/*
createTeam adds a team to the desk's workspace and returns its public
id.
*/
func createTeam(t *testing.T, d desk, name string) string {
	t.Helper()

	var id ids.UUID

	err := d.tc.Operator().QueryRow(t.Context(), "INSERT INTO teams (workspace_id, name) VALUES ($1, $2) RETURNING id",
		d.tc.Workspace.Workspace.ID, name).Scan(&id)
	require.NoError(t, err)

	return models.PublicID(models.PrefixTeam, id)
}
