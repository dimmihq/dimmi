package access_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
desks is a workspace with a second inbox, IT, a conversation in each
and the teammates the cases need: one on IT's list through a team, one
on it by name, one outside it and an admin.
*/
type desks struct {
	desk
	it, support          string
	inIT, inSupport      string
	itTeam               string
	byTeam, byName, none teammate
	admin                teammate
}

/*
withIT opens a desk with the IT inbox restricted to the IT team and one
teammate by name.
*/
func withIT(t *testing.T) desks {
	t.Helper()

	d := desks{desk: open(t)}
	d.support = models.PublicID(models.PrefixInbox, d.inbox.Inbox.ID)
	d.it = d.owner.PostJSON("/api/v1/inboxes", testkit.JSON{"name": "IT"}).AssertCreated().String("data.id")
	d.inIT = d.customerIn(t, d.it, "ivy")
	d.inSupport = d.customer(t, "sam")
	d.byTeam, d.byName = d.join(t, enums.RoleMember), d.join(t, enums.RoleMember)
	d.none, d.admin = d.join(t, enums.RoleMember), d.join(t, enums.RoleAdmin)
	d.itTeam = d.team(t, "IT")
	d.owner.PostJSON("/api/v1/teams/"+d.itTeam+"/members", testkit.JSON{
		"member_ids": []string{d.byTeam.public()},
	}).AssertOK()
	d.owner.PutJSON("/api/v1/inboxes/"+d.it+"/access", testkit.JSON{
		"mode": "restricted", "team_ids": []string{d.itTeam}, "member_ids": []string{d.byName.public()},
	}).AssertOK().AssertJSONPath("data.access.mode", "restricted")

	return d
}

/*
customerIn starts a conversation a customer wrote in the inbox.
*/
func (d desk) customerIn(t *testing.T, inbox, name string) string {
	t.Helper()

	return d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": inbox, "author": "contact", "subject": "About " + name, "body": "Words from " + name,
		"contact": testkit.JSON{"email": name + "@example.com", "name": name},
	}).AssertCreated().String("data.id")
}

/*
inboxIDs returns the ids of the inboxes the client lists.
*/
func inboxIDs(t *testing.T, c *testkit.Client) []string {
	t.Helper()

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	c.GetJSON("/api/v1/inboxes").AssertOK().Decode(&body)

	out := []string{}
	for _, row := range body.Data {
		out = append(out, row.ID)
	}

	return out
}

func TestInboxAccess(t *testing.T) {
	t.Parallel()

	all := "/api/v1/conversations?filter[view]=all&filter[status]=any"

	t.Run("a restricted inbox shows only to its list, its teams, owners and admins", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		for _, m := range []teammate{d.byTeam, d.byName, d.admin} {
			assert.ElementsMatch(t, []string{d.inIT, d.inSupport}, listed(t, m.client.GetJSON(all)))
			assert.ElementsMatch(t, []string{d.support, d.it}, inboxIDs(t, m.client))
			m.client.GetJSON("/api/v1/conversations/" + d.inIT).AssertOK()
		}

		assert.Equal(t, []string{d.inSupport}, listed(t, d.none.client.GetJSON(all)))
		assert.Equal(t, []string{d.support}, inboxIDs(t, d.none.client))
		d.none.client.GetJSON("/api/v1/inboxes/" + d.it).AssertNotFound()
		d.none.client.GetJSON("/api/v1/conversations/" + d.inIT).AssertNotFound()
		d.none.client.GetJSON("/api/v1/conversations/" + d.inIT + "/messages").AssertNotFound()
		d.none.client.GetJSON("/api/v1/conversation-counts").AssertOK().AssertJSONPath("data.views.all.open", 1.0)
	})

	t.Run("opening the inbox to everyone shows it to everyone again", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		opened := testkit.JSON{"mode": "everyone", "team_ids": []string{d.itTeam}}
		d.owner.PutJSON("/api/v1/inboxes/"+d.it+"/access", opened).
			AssertOK().AssertJSONPath("data.access.team_ids", []any{})

		assert.ElementsMatch(t, []string{d.inIT, d.inSupport}, listed(t, d.none.client.GetJSON(all)))
	})

	t.Run("leaving the team closes the inbox", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		d.owner.Delete("/api/v1/teams/" + d.itTeam + "/members/" + d.byTeam.public()).AssertNoContent()

		d.byTeam.client.GetJSON("/api/v1/conversations/" + d.inIT).AssertNotFound()
	})

	t.Run("a conversation assigned to someone shows to them in any inbox", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		d.assign(t, d.inIT, testkit.JSON{"assignee_id": d.none.public()})

		d.none.client.GetJSON("/api/v1/conversations/" + d.inIT).AssertOK()
		assert.Contains(t, listed(t, d.none.client.GetJSON(all)), d.inIT)
	})

	t.Run("a teammate mentioned in a note can read along", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		note := d.byName.client.PostJSON("/api/v1/conversations/"+d.inIT+"/notes", testkit.JSON{"body": "Over to you"}).
			AssertCreated().String("data.id")
		mention(t, d, note, d.none.member.ID)

		d.none.client.GetJSON("/api/v1/conversations/" + d.inIT).AssertOK()
		assert.Contains(t, listed(t, d.none.client.GetJSON(all)), d.inIT)
	})

	t.Run("nobody moves or starts a conversation in an inbox they cannot open", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)

		d.none.client.PatchJSON("/api/v1/conversations/"+d.inSupport, testkit.JSON{"inbox_id": d.it}).
			AssertValidationError("inbox_id")
		d.none.client.PostJSON("/api/v1/conversations", testkit.JSON{
			"inbox_id": d.it, "author": "contact", "subject": "Hi", "body": "Hello",
			"contact": testkit.JSON{"email": "zed@example.com"},
		}).AssertValidationError("inbox_id")
		d.byName.client.PatchJSON("/api/v1/conversations/"+d.inSupport, testkit.JSON{"inbox_id": d.it}).AssertOK()
	})

	t.Run("only those who manage inboxes change who may open one", func(t *testing.T) {
		t.Parallel()
		d := withIT(t)
		access := "/api/v1/inboxes/" + d.it + "/access"

		d.none.client.PutJSON(access, testkit.JSON{"mode": "everyone"}).AssertForbidden()
		d.owner.PutJSON(access, testkit.JSON{"mode": "secret"}).AssertValidationError("mode")
		d.owner.PutJSON(access, testkit.JSON{"mode": "restricted", "team_ids": []string{"team_nope"}}).
			AssertValidationError("team_ids")
		d.owner.PutJSON(access, testkit.JSON{
			"mode": "restricted", "member_ids": []string{models.PublicID(models.PrefixMember, ids.New())},
		}).AssertValidationError("members")
	})
}

/*
mention records that the note mentions the member, as writing a note
that names them does.
*/
func mention(t *testing.T, d desks, note string, member ids.UUID) {
	t.Helper()

	id, err := d.tc.App.IDs.Decode(string(models.PrefixMessage), note)
	require.NoError(t, err)

	_, err = d.tc.Operator().Exec(t.Context(), `
		INSERT INTO message_mentions (workspace_id, message_id, conversation_id, workspace_member_id)
		SELECT m.workspace_id, m.id, m.conversation_id, $1 FROM messages m
		WHERE m.workspace_id = $2 AND m.id = $3`,
		member, d.tc.Workspace.Workspace.ID, id)
	require.NoError(t, err)
}
