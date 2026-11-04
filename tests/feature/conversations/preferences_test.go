package conversations_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestInboxPreferences(t *testing.T) {
	t.Parallel()

	const path = "/api/v1/inbox-preferences"

	t.Run("starts empty and keeps what each save sends", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.GetJSON(path).AssertOK().AssertJSONPath("data.object", "inbox_preferences").
			AssertJSONPath("data.views", map[string]any{}).AssertJSONPath("data.updated_at", nil)

		d.owner.PatchJSON(path, testkit.JSON{
			"views": testkit.JSON{"mine": testkit.JSON{"sort": "waiting_since", "status": "any"}},
		}).AssertOK().AssertJSONPath("data.views.mine.sort", "waiting_since")
		d.owner.PatchJSON(path, testkit.JSON{"pinned_attributes": []string{"plan"}}).AssertOK()

		d.owner.GetJSON(path).AssertJSONPath("data.views.mine.status", "any").
			AssertJSONPath("data.pinned_attributes", []any{"plan"})
	})

	t.Run("keeps each member's own", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)

		sorted := testkit.JSON{"views": testkit.JSON{"all": testkit.JSON{"sort": "-priority"}}}
		d.owner.PatchJSON(path, sorted).AssertOK()

		bob.GetJSON(path).AssertJSONPath("data.views", map[string]any{})
	})

	t.Run("refuses views, sorts and statuses no list has", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		for _, views := range []testkit.JSON{
			{"nowhere": testkit.JSON{}},
			{"mine": testkit.JSON{"sort": "name"}},
			{"mine": testkit.JSON{"status": "pending"}},
			{"mine": testkit.JSON{"layout": "grid"}},
		} {
			d.owner.PatchJSON(path, testkit.JSON{"views": views}).AssertStatus(422).
				AssertJSONPath("code", "invalid_preferences")
		}
	})

	t.Run("refuses preferences over 64 KB", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.PatchJSON(path, testkit.JSON{"sidebar": testkit.JSON{"note": strings.Repeat("x", 65*1024)}}).
			AssertStatus(422).AssertJSONPath("code", "preferences_too_large")
	})

	t.Run("drops the views of a team that is gone", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		d.owner.PatchJSON(path, testkit.JSON{"views": testkit.JSON{
			team: testkit.JSON{"sort": "created_at"}, "mine": testkit.JSON{"sort": "created_at"},
		}}).AssertOK().AssertJSONPath("data.views."+team+".sort", "created_at")

		d.owner.Delete("/api/v1/teams/" + team).AssertNoContent()

		res := d.owner.GetJSON(path)
		res.AssertJSONMissing("data.views." + team)
		res.AssertJSONPath("data.views.mine.sort", "created_at")
	})

	t.Run("tells the member's other tabs", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		tab := d.tc.ListenAs(d.tc.Owner, d.tc.RealtimeURL())
		require.Nil(t, tab.Subscribe(appbroadcasting.MemberChannel(d.tc.Workspace.Owner.ID)).Error)

		sorted := testkit.JSON{"views": testkit.JSON{"mine": testkit.JSON{"sort": "created_at"}}}
		d.owner.PatchJSON(path, sorted).AssertOK()

		assert.Equal(t, "inbox_preferences.updated", tab.Publication().Event)
	})
}
