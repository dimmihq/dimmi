package conversations_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
team creates a team of the members in the desk's workspace and returns
its public id.
*/
func (d desk) team(t *testing.T, name string, members ...models.WorkspaceMember) string {
	t.Helper()

	team, err := factories.Team(t.Context(), d.tc.App, factories.TeamOf{
		Workspace: d.tc.Workspace.Workspace, Name: name, Members: members,
	})
	require.NoError(t, err)

	return models.PublicID(models.PrefixTeam, team.ID)
}

func TestMentions(t *testing.T) {
	t.Parallel()

	t.Run("records each mentioned teammate once, a team as its members, never the writer", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		created := record[appevents.MessageCreated](d.tc)
		_, bob := d.join(t, enums.RoleMember)
		_, cleo := d.join(t, enums.RoleMember)
		_, gone := d.join(t, enums.RoleMember)
		billing := d.team(t, "Billing", bob, cleo, gone, d.tc.Workspace.Owner)
		require.NoError(t, factories.Deactivate(t.Context(), d.tc.App, gone))
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")

		body := "<@" + memberID(bob) + "> and <@" + billing + "> and me <@" + d.ownerID() + "> and <@" +
			memberID(gone) + "> and <@mem_nobody>"
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": body}).AssertCreated().
			AssertJSONPath("data.mentions.0.name", "Billing")

		d.tc.Table("message_mentions").AssertCount(2, nil)
		d.tc.Table("message_mentions").AssertCount(1, testkit.Row{"workspace_member_id": bob.ID, "via_team_id": nil})
		d.tc.Table("message_mentions").AssertCount(1, testkit.Row{"workspace_member_id": cleo.ID})
		d.tc.Drain()
		require.NotEmpty(t, created.all())
		assert.ElementsMatch(t, []ids.UUID{bob.ID, cleo.ID}, created.all()[len(created.all())-1].MentionedMemberIDs)
	})

	t.Run("leaves team tokens as text while team mentions are off", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, bob := d.join(t, enums.RoleMember)
		billing := d.team(t, "Billing", bob)
		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"team_mentions_enabled": false}).AssertOK().
			AssertJSONPath("data.team_mentions_enabled", false)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")

		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Hey <@" + billing + ">"}).AssertCreated().
			AssertJSONPath("data.mentions", []any{})

		d.tc.Table("message_mentions").AssertCount(0, nil)
		d.owner.GetJSON(path+"/mentionables").AssertOK().AssertJSONPath("data.teams", []any{})
	})

	t.Run("adds only new mentions on edit and drops those taken out", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		updated := record[appevents.MessageUpdated](d.tc)
		_, bob := d.join(t, enums.RoleMember)
		_, cleo := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")
		note := d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "<@" + memberID(bob) + ">"}).AssertCreated().
			String("data.id")

		d.owner.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "<@" + memberID(cleo) + "> <@" +
			memberID(bob) + ">"}).AssertOK()
		d.owner.PatchJSON(path+"/messages/"+note, testkit.JSON{"body": "<@" + memberID(cleo) + ">"}).AssertOK()

		d.tc.Table("message_mentions").AssertCount(1, nil)
		d.tc.Table("message_mentions").AssertCount(1, testkit.Row{"workspace_member_id": cleo.ID})
		d.tc.Drain()
		require.Len(t, updated.all(), 2)
		assert.Equal(t, []ids.UUID{cleo.ID}, updated.all()[0].AddedMentionedMemberIDs)
		assert.Empty(t, updated.all()[1].AddedMentionedMemberIDs)
	})

	t.Run("never reads mentions in a reply", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, bob := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "<@" + memberID(bob) + ">"}).AssertCreated()

		d.tc.Table("message_mentions").AssertCount(0, nil)
	})

	t.Run("lists and counts a mention for the teammate until they read it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bobClient, bob := d.join(t, enums.RoleMember)
		conv := d.customer(t, "ann", "Refund", "Refund please")
		path := "/api/v1/conversations/" + conv
		d.customer(t, "ben", "Other", "Hi")
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "Look <@" + memberID(bob) + ">"}).AssertCreated()

		bobClient.GetJSON("/api/v1/conversations?filter[view]=mentions").AssertOK().
			AssertJSONPath("data.#", 1).AssertJSONPath("data.0.id", conv)
		bobClient.GetJSON("/api/v1/conversation-counts").AssertOK().AssertJSONPath("data.views.mentions.unread", 1)
		d.owner.GetJSON("/api/v1/conversations?filter[view]=mentions").AssertOK().AssertJSONPath("data.#", 0)

		bobClient.PatchJSON(path+"/read-state", testkit.JSON{}).AssertOK()
		bobClient.GetJSON("/api/v1/conversation-counts").AssertOK().AssertJSONPath("data.views.mentions.unread", 0)
	})

	t.Run("keeps a mention of someone who cannot see the conversation out of their view", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bobClient, bob := d.join(t, enums.RoleMember)
		d.owner.PatchJSON("/api/v1/members/"+memberID(bob), testkit.JSON{"conversation_access": "assigned"}).AssertOK()
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": d.ownerID()}).AssertOK()

		d.owner.GetJSON(path+"/mentionables?q="+url.QueryEscape(bobName(t, d, bob))).AssertOK().
			AssertJSONPath(`data.members.#(id=="`+memberID(bob)+`").can_see`, false)
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "<@" + memberID(bob) + ">"}).AssertCreated()

		d.tc.Table("message_mentions").AssertCount(1, testkit.Row{"workspace_member_id": bob.ID})
		bobClient.GetJSON("/api/v1/conversations?filter[view]=mentions").AssertOK().AssertJSONPath("data.#", 0)
		bobClient.GetJSON("/api/v1/conversation-counts").AssertOK().AssertJSONPath("data.views.mentions.unread", 0)
	})

	t.Run("finds who to mention by a word of their name", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, bob := d.join(t, enums.RoleMember)
		d.team(t, "Billing Squad", bob)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Refund", "Refund please")

		d.owner.GetJSON(path+"/mentionables?q=squad").AssertOK().
			AssertJSONPath("data.teams.0.name", "Billing Squad").AssertJSONPath("data.teams.0.member_count", 1)
		d.owner.GetJSON(path+"/mentionables").AssertOK().
			AssertJSONPath("data.members.#", 2).AssertJSONPath("data.members.0.can_see", true)
		d.owner.GetJSON("/api/v1/conversations/conv_nope/mentionables").AssertNotFound()
	})
}

/*
bobName returns the member's user's name, which the factory chose.
*/
func bobName(t *testing.T, d desk, m models.WorkspaceMember) string {
	t.Helper()

	var name string
	require.NoError(t, d.tc.Operator().QueryRow(t.Context(),
		"SELECT u.name FROM users u JOIN workspace_members wm ON wm.user_id = u.id WHERE wm.id = $1", m.ID).Scan(&name))

	return name
}
