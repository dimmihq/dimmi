package conversations_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestConversationViewers(t *testing.T) {
	t.Parallel()

	const list = "/api/v1/conversations?filter[view]=all&include=viewers"

	t.Run("lists who views each row, leaving the caller out", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Refund", "Please")
		bob, member := d.joinUser(t)
		url := d.tc.RealtimeURL()
		watching := d.tc.ListenAs(bob, url)
		require.Nil(t, watching.Subscribe("conversation:"+id).Error)

		require.Eventually(t, func() bool {
			return d.owner.GetJSON(list).String("data.0.viewers.0.id") == memberID(member)
		}, testkit.RealtimeWait, 20*time.Millisecond)
		d.owner.GetJSON(list).AssertJSONPath("data.0.viewers.0.name", bob.Name).
			AssertJSONPath("data.0.viewers.#", 1)
		in(d.tc, bob).GetJSON(list).AssertJSONPath("data.0.viewers", []any{})
	})

	t.Run("leaves viewers out unless asked, or when the workspace hides them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		d.customer(t, "ann", "Refund", "Please")

		d.owner.GetJSON(list).AssertJSONPath("data.0.viewers", []any{})
		d.owner.GetJSON("/api/v1/conversations?filter[view]=all").AssertJSONMissing("data.0.viewers")

		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"show_viewers_on_list": false}).AssertOK().
			AssertJSONPath("data.show_viewers_on_list", false)
		d.owner.GetJSON(list).AssertJSONMissing("data.0.viewers")
	})

	t.Run("announces who views a conversation to the workspace's lists", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		id := d.customer(t, "ann", "Refund", "Please")
		bob, member := d.joinUser(t)
		url := d.tc.RealtimeURL()
		lists := d.tc.ListenAs(d.tc.Owner, url)
		require.Nil(t, lists.Subscribe(appbroadcasting.WorkspaceChannel(d.tc.Workspace.Workspace.ID)).Error)

		require.Nil(t, d.tc.ListenAs(bob, url).Subscribe("conversation:"+id).Error)

		changed := make(chan testkit.RealtimePublication, 1)
		go func() {
			for {
				if p := lists.Publication(); p.Event == "conversation.viewers_changed" {
					changed <- p
					return
				}
			}
		}()

		var got testkit.RealtimePublication
		require.Eventually(t, func() bool {
			d.tc.Clock.Advance(time.Second)
			select {
			case got = <-changed:
				return true
			default:
				return false
			}
		}, testkit.RealtimeWait, 20*time.Millisecond)

		var data struct {
			ID      string `json:"id"`
			Viewers []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"viewers"`
		}
		require.NoError(t, json.Unmarshal(got.Data, &data))
		assert.Equal(t, id, data.ID)
		require.Len(t, data.Viewers, 1)
		assert.Equal(t, memberID(member), data.Viewers[0].ID)
	})
}

/*
joinUser adds a new member to the desk's workspace and returns their
user and membership.
*/
func (d desk) joinUser(t *testing.T) (models.User, models.WorkspaceMember) {
	t.Helper()

	user := d.tc.CreateUser()
	member, err := factories.Member(t.Context(), d.tc.App, factories.Membership{
		Workspace: d.tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
	})
	require.NoError(t, err)

	return user, member
}
