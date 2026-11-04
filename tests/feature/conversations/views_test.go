package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestTeamAndInboxViews(t *testing.T) {
	t.Parallel()

	t.Run("lists a team's and an inbox's conversations, and counts each holder", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, bob := d.join(t, enums.RoleMember)
		team := d.owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Billing"}).AssertCreated().String("data.id")
		billing := d.customer(t, "ann", "Invoice", "Hi")
		other := d.customer(t, "bea", "Login", "Hi")
		d.owner.PatchJSON("/api/v1/conversations/"+billing, testkit.JSON{
			"team_id": team, "assignee_id": memberID(bob),
		}).AssertOK()

		assert.Equal(t, []string{billing}, listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=team:"+team)))
		assert.ElementsMatch(t, []string{billing, other},
			listed(d.owner.GetJSON("/api/v1/conversations?filter[view]=inbox:"+d.inboxID())))
		d.owner.GetJSON("/api/v1/conversations?filter[view]=team:nope").AssertStatus(400)

		d.owner.GetJSON("/api/v1/conversation-counts").AssertOK().
			AssertJSONPath("data.views.teams."+team+".open", 1).
			AssertJSONPath("data.views.members."+memberID(bob)+".open", 1)
	})
}
