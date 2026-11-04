package assignment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
)

func TestMemberAvailability(t *testing.T) {
	t.Parallel()

	t.Run("lets a member go away, reassign and come back", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)

		away := m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away"}).AssertOK()
		away.AssertJSONPath("data.availability", "away")
		away.AssertJSONPath("data.away_reassign", false)
		assert.NotEmpty(t, away.String("data.away_since"))

		reassigning := m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"away_reassign": true}).AssertOK()
		reassigning.AssertJSONPath("data.away_reassign", true)
		assert.Equal(t, away.String("data.away_since"), reassigning.String("data.away_since"))

		back := m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online"}).AssertOK()
		back.AssertJSONPath("data.availability", "online")
		back.AssertJSONPath("data.away_reassign", false)
		back.AssertJSONPath("data.away_since", nil)

		m.client.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.member.availability", "online")
	})

	t.Run("refuses reassigning while online", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)

		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"away_reassign": true}).
			AssertStatus(422).AssertJSONPath("code", "away_reassign_requires_away")
		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online", "away_reassign": true}).
			AssertStatus(422)
		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "busy"}).
			AssertValidationError("availability")
	})

	t.Run("needs the permission for someone else and audits it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)
		other := d.join(t)
		logs := "/api/v1/audit-logs?filter[event]=member.availability_changed"

		other.client.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{"availability": "away"}).AssertForbidden()

		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away"}).AssertOK()
		d.owner.GetJSON(logs).AssertOK().AssertJSONPath("data.#", 0)

		d.owner.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{"availability": "online"}).AssertOK()
		d.owner.GetJSON(logs).AssertOK().AssertJSONPath("data.#", 1)

		d.owner.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{"availability": "online"}).AssertOK()
		d.owner.GetJSON(logs).AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("sets capacity only with the permission and lifts it with null", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)
		path := "/api/v1/members/" + m.public()

		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"conversation_capacity": 5}).AssertForbidden()

		set := d.owner.PatchJSON(path, testkit.JSON{
			"conversation_capacity": 10, "capacity_by_channel": testkit.JSON{"email": 4},
		}).AssertOK()
		set.AssertJSONPath("data.conversation_capacity", 10)
		set.AssertJSONPath("data.capacity_by_channel.email", 4)

		d.owner.PatchJSON(path, testkit.JSON{"conversation_capacity": nil}).AssertOK().
			AssertJSONPath("data.conversation_capacity", nil)
		d.owner.PatchJSON(path, testkit.JSON{"conversation_capacity": 501}).
			AssertValidationError("conversation_capacity")
		d.owner.PatchJSON(path, testkit.JSON{"capacity_by_channel": testkit.JSON{"fax": 3}}).AssertStatus(422)
	})

	t.Run("lets a token that writes conversations set its own status", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)
		workspace := models.PublicID(models.PrefixWorkspace, d.tc.Workspace.Workspace.ID)
		token := d.tc.ActingAsToken(m.member, "conversations:write").WithHeader("Dimmi-Workspace", workspace)
		reader := d.tc.ActingAsToken(m.member, "conversations:read").WithHeader("Dimmi-Workspace", workspace)

		token.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away"}).AssertOK()
		reader.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online"}).AssertForbidden()
	})

	t.Run("keeps the reply rule per member", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)

		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"assign_on_reply": "never"}).AssertOK().
			AssertJSONPath("data.assign_on_reply", "never")
		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"assign_on_reply": "sometimes"}).
			AssertValidationError("assign_on_reply")
	})
}

func TestAwayReassign(t *testing.T) {
	t.Parallel()

	t.Run("hands a conversation back to its team when the customer writes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)
		team := d.team(t, "Billing")
		id := d.customer(t, "ann")
		d.assign(t, id, testkit.JSON{"assignee_id": m.public(), "team_id": team})
		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away", "away_reassign": true}).AssertOK()

		d.writes(t, id, "away:1", false)

		shown := d.show(t, id)
		shown.AssertJSONPath("data.assignee", nil)
		shown.AssertJSONPath("data.team.id", team)

		d.writes(t, id, "away:2", false)
		d.show(t, id).AssertJSONPath("data.team.id", team)

		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "online"}).AssertOK()
		d.show(t, id).AssertJSONPath("data.assignee", nil)
	})

	t.Run("leaves it with an away member who did not ask, and for automatic mail", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t)
		other := d.join(t)
		kept, auto := d.customer(t, "kim"), d.customer(t, "lee")
		d.assign(t, kept, testkit.JSON{"assignee_id": m.public()})
		d.assign(t, auto, testkit.JSON{"assignee_id": other.public()})
		m.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away"}).AssertOK()
		other.client.PatchJSON("/api/v1/members/me", testkit.JSON{"availability": "away", "away_reassign": true}).
			AssertOK()

		d.writes(t, kept, "kept:1", false)
		d.writes(t, auto, "auto:1", true)

		d.show(t, kept).AssertJSONPath("data.assignee.id", m.public())
		d.show(t, auto).AssertJSONPath("data.assignee.id", other.public())
	})
}

func TestAssignOnReply(t *testing.T) {
	t.Parallel()

	t.Run("takes an unassigned conversation by replying unless the member never does", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		taker, lead := d.join(t), d.join(t)
		lead.client.PatchJSON("/api/v1/members/me", testkit.JSON{"assign_on_reply": "never"}).AssertOK()
		first, second := d.customer(t, "ann"), d.customer(t, "bob")

		lead.client.PostJSON("/api/v1/conversations/"+first+"/replies", testkit.JSON{"body": "Looking"}).AssertCreated()
		d.show(t, first).AssertJSONPath("data.assignee", nil)

		taker.client.PostJSON("/api/v1/conversations/"+second+"/replies", testkit.JSON{"body": "On it"}).AssertCreated()
		d.show(t, second).AssertJSONPath("data.assignee.id", taker.public())
	})

	t.Run("follows an inbox that turned taking off, unless the member always takes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		follower, always := d.join(t), d.join(t)
		always.client.PatchJSON("/api/v1/members/me", testkit.JSON{"assign_on_reply": "always"}).AssertOK()
		_, err := d.tc.Operator().Exec(t.Context(),
			`UPDATE inboxes SET settings = settings || '{"assign_on_reply": false}' WHERE id = $1`, d.inbox.Inbox.ID)
		assert.NoError(t, err)
		first, second := d.customer(t, "ann"), d.customer(t, "bob")

		follower.client.PostJSON("/api/v1/conversations/"+first+"/replies", testkit.JSON{"body": "Hi"}).AssertCreated()
		d.show(t, first).AssertJSONPath("data.assignee", nil)

		always.client.PostJSON("/api/v1/conversations/"+second+"/replies", testkit.JSON{"body": "Hi"}).AssertCreated()
		d.show(t, second).AssertJSONPath("data.assignee.id", always.public())
	})

	t.Run("gives a conversation reopened by hand to a member who always takes", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		always, plain := d.join(t), d.join(t)
		always.client.PatchJSON("/api/v1/members/me", testkit.JSON{"assign_on_reply": "always"}).AssertOK()
		first, second := d.customer(t, "ann"), d.customer(t, "bob")
		d.assign(t, first, testkit.JSON{"status": "closed"})
		d.assign(t, second, testkit.JSON{"status": "closed"})

		always.client.PatchJSON("/api/v1/conversations/"+first, testkit.JSON{"status": "open"}).AssertOK()
		plain.client.PatchJSON("/api/v1/conversations/"+second, testkit.JSON{"status": "open"}).AssertOK()

		d.show(t, first).AssertJSONPath("data.assignee.id", always.public())
		d.show(t, second).AssertJSONPath("data.assignee", nil)
	})
}
