package access_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
holding is one way a conversation can sit and be held, with the scope
looking at it.
*/
type holding struct {
	scope        conversations.Scope
	inbox        ids.UUID
	member, team *ids.UUID
}

/*
holdings returns every access mode and shape of list against every way
of holding a conversation, in an inbox the scope opens and one it may
not.
*/
func holdings() []holding {
	me, other := ids.New(), ids.New()
	listedTeam, otherTeam := ids.New(), ids.New()
	open, hidden := ids.New(), ids.New()

	var out []holding

	for _, mode := range []enums.ConversationAccess{
		enums.ConversationAccessAll, enums.ConversationAccessAssigned, enums.ConversationAccessTeams,
		enums.ConversationAccessAllExceptTeams,
	} {
		for _, list := range [][]ids.UUID{nil, {listedTeam}} {
			for _, member := range []*ids.UUID{nil, &me, &other} {
				for _, team := range []*ids.UUID{nil, &listedTeam, &otherTeam} {
					for _, inbox := range []ids.UUID{open, hidden} {
						scope := conversations.Scope{
							Mode: mode, MemberID: &me, TeamIDs: list, HiddenInboxes: []ids.UUID{hidden},
						}
						out = append(out, holding{scope, inbox, member, team})
						scope.Unassigned = true
						out = append(out, holding{scope, inbox, member, team})
					}
				}
			}
		}
	}

	return out
}

func TestScopeEqualsSQLFunction(t *testing.T) {
	t.Parallel()

	tc := open(t).tc

	for _, h := range holdings() {
		params := h.scope.Params()

		var viaSQL *bool

		err := tc.App.DB.QueryRow(t.Context(), "SELECT dimmi_conversation_visible($1, $2, $3, $4, $5, $6, $7, $8)",
			h.member, h.team, h.inbox, params.Mode, params.Viewer, params.Teams, params.Unassigned,
			params.HiddenInboxes).Scan(&viaSQL)
		require.NoError(t, err)

		name := fmt.Sprintf("%s list=%d unassigned=%v member=%v team=%v hidden=%v",
			h.scope.Mode, len(h.scope.TeamIDs), h.scope.Unassigned, h.member != nil, h.team != nil,
			!h.scope.OpensInbox(h.inbox))
		held := models.Conversation{InboxID: h.inbox, AssigneeMemberID: h.member, AssigneeTeamID: h.team}

		assert.Equal(t, viaSQL != nil && *viaSQL, h.scope.Contains(held), name)
	}
}

func TestScopeRules(t *testing.T) {
	t.Parallel()

	t.Run("an actor with no person behind it sees everything, trash included", func(t *testing.T) {
		t.Parallel()

		scope := authz.WorkspaceScope()
		deleted := models.Conversation{DeletedAt: new(longAgo())}

		assert.True(t, scope.Contains(deleted))
		assert.Nil(t, scope.Params().Viewer)
	})

	t.Run("a deleted conversation shows only to those who reach the trash", func(t *testing.T) {
		t.Parallel()

		deleted := models.Conversation{DeletedAt: new(longAgo())}
		me := ids.New()

		assert.False(t, authz.MemberScope(models.WorkspaceMember{ID: me, ConversationAccess: "all"}, nil, false).
			Contains(deleted))
		assert.True(t, authz.MemberScope(models.WorkspaceMember{ID: me, ConversationAccess: "all"}, nil, true).
			Contains(deleted))
	})

	t.Run("a mode nobody knows shows only the member's own", func(t *testing.T) {
		t.Parallel()

		me := ids.New()
		scope := conversations.Scope{Mode: "", MemberID: &me}

		assert.True(t, scope.Sees(ids.New(), &me, nil))
		assert.False(t, scope.Sees(ids.New(), nil, nil))
	})
}

func TestRestrictedMemberSurfaces(t *testing.T) {
	t.Parallel()

	d := open(t)
	billing, support := d.team(t, "Billing"), d.team(t, "Support")

	onBilling := d.customer(t, "bea")
	d.assign(t, onBilling, testkit.JSON{"team_id": billing})

	onSupport := d.customer(t, "sam")
	d.assign(t, onSupport, testkit.JSON{"team_id": support})

	nobodys := d.customer(t, "nia")

	number := func(id string) string {
		return d.owner.GetJSON("/api/v1/conversations/" + id).AssertOK().String("data.number")
	}
	all := "/api/v1/conversations?filter[view]=all&filter[status]=any"

	type shape struct {
		name    string
		access  testkit.JSON
		visible []string
		hidden  []string
	}

	cases := []shape{
		{"all", testkit.JSON{"conversation_access": "all"}, []string{onBilling, onSupport, nobodys}, nil},
		{"assigned", testkit.JSON{"conversation_access": "assigned"}, nil, []string{onBilling, onSupport, nobodys}},
		{
			"assigned and unassigned",
			testkit.JSON{
				"conversation_access": "assigned", "conversation_access_unassigned": true,
			},
			[]string{nobodys},
			[]string{onBilling, onSupport},
		},
		{
			"teams",
			testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{billing}},
			[]string{onBilling},
			[]string{onSupport, nobodys},
		},
		{
			"all except teams",
			testkit.JSON{
				"conversation_access": "all_except_teams", "access_team_ids": []string{billing},
			},
			[]string{onSupport, nobodys},
			[]string{onBilling},
		},
	}

	for _, c := range cases {
		t.Run(c.name+" shows only what the mode allows on every surface", func(t *testing.T) {
			m := d.join(t, enums.RoleMember)
			d.restrict(t, m, c.access)

			assert.ElementsMatch(t, c.visible, listed(t, m.client.GetJSON(all)))

			for _, id := range c.visible {
				m.client.GetJSON("/api/v1/conversations/" + id).AssertOK()
				m.client.GetJSON("/api/v1/conversations/" + id + "/messages").AssertOK()
				m.client.GetJSON("/api/v1/conversations/" + id + "/neighbors?filter[view]=all").AssertOK()
			}

			for _, id := range c.hidden {
				base := "/api/v1/conversations/" + id
				m.client.GetJSON(base).AssertNotFound()
				m.client.GetJSON("/api/v1/conversations/" + number(id)).AssertNotFound()
				m.client.GetJSON(base + "/messages").AssertNotFound()
				m.client.GetJSON(base + "/neighbors?filter[view]=all").AssertNotFound()
				m.client.GetJSON(base + "/read-state").AssertNotFound()
				m.client.PatchJSON(base+"/read-state", testkit.JSON{}).AssertNotFound()
				m.client.PatchJSON(base, testkit.JSON{"priority": "high"}).AssertNotFound()
				m.client.PostJSON(base+"/notes", testkit.JSON{"body": "hi"}).AssertNotFound()
				m.client.PostJSON(base+"/replies", testkit.JSON{"body": "hi"}).AssertNotFound()
				m.client.PostJSON(base+"/typing-indicators", testkit.JSON{"state": "start", "composer": "reply"}).
					AssertNotFound()

				res := m.client.GetJSON("/api/v1/search?q=%23" + number(id)).AssertOK()
				res.AssertJSONPath("data.jump", nil)
			}

			counts := m.client.GetJSON("/api/v1/conversation-counts").AssertOK()
			counts.AssertJSONPath("data.views.all.open", float64(len(c.visible)))
		})
	}

	t.Run("search finds only visible conversations by their words", func(t *testing.T) {
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{billing}})

		bea := m.client.GetJSON("/api/v1/search?q=bea").AssertOK()
		sam := m.client.GetJSON("/api/v1/search?q=sam").AssertOK()

		assert.Contains(t, string(bea.Body), onBilling)
		assert.NotContains(t, string(sam.Body), onSupport)
	})

	t.Run("neighbors skip what the member cannot see", func(t *testing.T) {
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{billing}})

		res := m.client.GetJSON("/api/v1/conversations/" + onBilling + "/neighbors?filter[view]=all").AssertOK()

		assert.NotContains(t, string(res.Body), onSupport)
		assert.NotContains(t, string(res.Body), nobodys)
	})

	t.Run("a customer's list shows only visible rows", func(t *testing.T) {
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		contact := d.owner.GetJSON("/api/v1/conversations/" + onBilling).AssertOK().String("data.contact.id")

		assert.Empty(t, listed(t, m.client.GetJSON(all+"&filter[contact]="+contact)))
		assert.Len(t, listed(t, d.owner.GetJSON(all+"&filter[contact]="+contact)), 1)
	})
}

func TestOwnAssignment(t *testing.T) {
	t.Parallel()

	t.Run("assigning a member grants sight and reassigning takes it away", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		id := d.customer(t, "ann")

		m.client.GetJSON("/api/v1/conversations/" + id).AssertNotFound()

		d.assign(t, id, testkit.JSON{"assignee_id": m.public()})
		m.client.GetJSON("/api/v1/conversations/" + id).AssertOK()
		m.client.PostJSON("/api/v1/conversations/"+id+"/notes", testkit.JSON{"body": "on it"}).AssertCreated()

		d.assign(t, id, testkit.JSON{"assignee_id": nil})
		m.client.GetJSON("/api/v1/conversations/" + id).AssertNotFound()
	})

	t.Run("a deleted team narrows teams and widens all except teams", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.team(t, "Gone soon")
		id := d.customer(t, "ann")
		d.assign(t, id, testkit.JSON{"team_id": team})

		narrow := d.join(t, enums.RoleMember)
		d.restrict(t, narrow, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{team}})
		wide := d.join(t, enums.RoleMember)
		d.restrict(t, wide, testkit.JSON{"conversation_access": "all_except_teams", "access_team_ids": []string{team}})

		narrow.client.GetJSON("/api/v1/conversations/" + id).AssertOK()
		wide.client.GetJSON("/api/v1/conversations/" + id).AssertNotFound()

		d.owner.Delete("/api/v1/teams/" + team).AssertNoContent()
		d.tc.Drain()

		narrow.client.GetJSON("/api/v1/conversations/" + id).AssertNotFound()
		wide.client.GetJSON("/api/v1/conversations/" + id).AssertOK()
	})
}

func TestTokensFollowTheirMember(t *testing.T) {
	t.Parallel()

	d := open(t)
	m := d.join(t, enums.RoleMember)
	id := d.customer(t, "ann")
	d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})

	token := d.tc.ActingAsToken(m.member, "conversations:read")
	ownerToken := d.tc.ActingAsToken(d.tc.Workspace.Owner, "conversations:read")

	token.GetJSON("/api/v1/conversations/" + id).AssertNotFound()
	assert.Empty(t, listed(t, token.GetJSON("/api/v1/conversations?filter[view]=all")))
	ownerToken.GetJSON("/api/v1/conversations/" + id).AssertOK()
	assert.Len(t, listed(t, ownerToken.GetJSON("/api/v1/conversations?filter[view]=all")), 1)
}
