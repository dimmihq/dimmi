package access_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
workspaceUUID returns the row id of the desk's workspace.
*/
func (d desk) workspaceUUID() ids.UUID {
	return d.tc.Workspace.Workspace.ID
}

/*
decode reads a conversation's public id.
*/
func (d desk) decode(t *testing.T, public string) ids.UUID {
	t.Helper()

	id, err := d.tc.App.IDs.Decode(string(models.PrefixConversation), public)
	require.NoError(t, err)

	return id
}

func TestRealtimeAccess(t *testing.T) {
	t.Parallel()

	t.Run("refuses a restricted member the list channel and admits one who sees everything", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		channel := appbroadcasting.WorkspaceConversationsChannel(d.workspaceUUID())

		assert.NotNil(t, d.tc.ListenAs(m.user, url).Subscribe(channel).Error)
		assert.Nil(t, d.tc.ListenAs(d.tc.Owner, url).Subscribe(channel).Error)
	})

	t.Run("keeps conversation changes off the workspace channel", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})

		c := d.tc.ListenAs(m.user, url)
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(d.workspaceUUID())).Error)

		id := d.customer(t, "ann")
		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"priority": true}).AssertOK()
		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).AssertOK()

		assert.Equal(t, "workspace.updated", c.Publication().Event)
	})

	t.Run("sends a member copies only of what they see", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		billing, support := d.team(t, "Billing"), d.team(t, "Support")
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{billing}})
		hidden, seen := d.customer(t, "sam"), d.customer(t, "bea")
		d.assign(t, hidden, testkit.JSON{"team_id": support})
		d.assign(t, seen, testkit.JSON{"team_id": billing})

		c := d.tc.ListenAs(m.user, url)
		require.Nil(t, c.Subscribe(appbroadcasting.MemberChannel(m.member.ID)).Error)

		d.owner.PatchJSON("/api/v1/conversations/"+hidden, testkit.JSON{"priority": true}).AssertOK()
		d.owner.PatchJSON("/api/v1/conversations/"+seen, testkit.JSON{"priority": true}).AssertOK()

		copied := c.Publication()
		assert.Equal(t, "conversation.priority_changed", copied.Event)
		assert.Contains(t, string(copied.Data), seen)
		assert.NotContains(t, string(copied.Data), hidden)
	})

	t.Run("tells a member reassigned away to drop it and ends their subscription", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		m := d.join(t, enums.RoleMember)
		other := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		id := d.customer(t, "ann")
		d.assign(t, id, testkit.JSON{"assignee_id": m.public()})

		mine := d.tc.ListenAs(m.user, url)
		require.Nil(t, mine.Subscribe(appbroadcasting.MemberChannel(m.member.ID)).Error)
		thread := d.tc.ListenAs(m.user, url)
		require.Nil(t, thread.Subscribe(appbroadcasting.ConversationChannel(d.decode(t, id))).Error)

		d.assign(t, id, testkit.JSON{"assignee_id": other.public()})

		hidden := mine.Publication()
		assert.Equal(t, appbroadcasting.EventConversationHidden, hidden.Event)
		assert.Contains(t, string(hidden.Data), id)
		assert.Equal(t, appbroadcasting.ConversationChannel(d.decode(t, id)), thread.Unsubscribed())
	})

	t.Run("closes a member's connections when their access changes, quickly", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		m := d.join(t, enums.RoleMember)
		c := d.tc.ListenAs(m.user, url)
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceConversationsChannel(d.workspaceUUID())).Error)

		wall := clock.System()
		started := wall.Now()
		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})

		assert.Equal(t, 4009, c.Closed())
		assert.Less(t, wall.Since(started), 5*time.Second)

		again := d.tc.ListenAs(m.user, url)
		assert.NotNil(t, again.Subscribe(appbroadcasting.WorkspaceConversationsChannel(d.workspaceUUID())).Error)
	})

	t.Run("stops copies after a member gets full sight again", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		url := d.tc.RealtimeURL()
		m := d.join(t, enums.RoleMember)
		d.restrict(t, m, testkit.JSON{"conversation_access": "all_except_teams"})
		id := d.customer(t, "ann")
		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"priority": true}).AssertOK()
		d.restrict(t, m, testkit.JSON{"conversation_access": "all"})

		c := d.tc.ListenAs(m.user, url)
		require.Nil(t, c.Subscribe(appbroadcasting.MemberChannel(m.member.ID)).Error)
		d.owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"priority": false}).AssertOK()
		d.restrict(t, d.join(t, enums.RoleMember), testkit.JSON{"conversation_access": "assigned"})
		d.owner.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{"conversation_access": "assigned"}).AssertOK()

		assert.Equal(t, "member.access_changed", c.Publication().Event)
	})
}

func TestAudienceBudget(t *testing.T) {
	t.Parallel()

	t.Run("decides for two hundred restricted members within five milliseconds", func(t *testing.T) {
		t.Parallel()

		team := ids.New()
		members := make([]appbroadcasting.RestrictedMember, 200)

		for i := range members {
			members[i] = appbroadcasting.RestrictedMember{
				MemberID: ids.New(), UserID: ids.New(), Mode: "teams", TeamIDs: []ids.UUID{ids.New(), team},
			}
		}

		held := appbroadcasting.Holding{Team: &team}
		wall := clock.System()
		started := wall.Now()
		audience := appbroadcasting.AudienceOf(members, held, held)

		assert.Less(t, wall.Since(started), 5*time.Millisecond)
		assert.Len(t, audience.Copies, 200)
	})
}
