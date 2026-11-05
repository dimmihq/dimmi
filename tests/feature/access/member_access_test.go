package access_test

import (
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
longAgo is a moment long past, for rows that need one.
*/
func longAgo() time.Time {
	return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestUpdateMemberAccess(t *testing.T) {
	t.Parallel()

	t.Run("sets the mode, the list and the unassigned switch and shows them", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		team := d.team(t, "Billing")

		res := d.owner.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{
			"conversation_access": "teams", "access_team_ids": []string{team}, "conversation_access_unassigned": true,
		}).AssertOK()

		res.AssertJSONPath("data.conversation_access", "teams")
		res.AssertJSONPath("data.conversation_access_unassigned", true)
		res.AssertJSONPath("data.access_team_ids.0", team)

		d.owner.GetJSON("/api/v1/members/"+m.public()).AssertOK().AssertJSONPath("data.access_team_ids.0", team)
		d.owner.GetJSON("/api/v1/members").AssertOK()

		me := m.client.GetJSON("/api/v1/me").AssertOK()
		me.AssertJSONPath("data.member.conversation_access", "teams")
		me.AssertJSONPath("data.member.conversation_access_unassigned", true)
		me.AssertJSONPath("data.member.access_team_ids.0", team)
	})

	t.Run("replaces the list and empties it for a mode that reads none", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		one, two := d.team(t, "One"), d.team(t, "Two")

		d.restrict(t, m, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{one, two}})
		d.restrict(t, m, testkit.JSON{"access_team_ids": []string{two}})
		d.owner.GetJSON("/api/v1/members/"+m.public()).AssertJSONPath("data.access_team_ids.#", 1)

		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		d.owner.GetJSON("/api/v1/members/"+m.public()).AssertJSONPath("data.access_team_ids.#", 0)
	})

	t.Run("records the change once and ignores the same value", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		logs := "/api/v1/audit-logs?filter[event]=member.access_changed"

		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		d.owner.GetJSON(logs).AssertOK().AssertJSONPath("data.#", 1)

		d.restrict(t, m, testkit.JSON{"conversation_access": "assigned"})
		d.owner.GetJSON(logs).AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("refuses changing one's own access whatever the role", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		admin := d.join(t, enums.RoleAdmin)

		admin.client.PatchJSON("/api/v1/members/"+admin.public(), testkit.JSON{"conversation_access": "all"}).
			AssertForbidden()
		admin.client.PatchJSON("/api/v1/members/me", testkit.JSON{"conversation_access": "all"}).AssertForbidden()
		d.owner.PatchJSON("/api/v1/members/me", testkit.JSON{"conversation_access": "assigned"}).AssertForbidden()
	})

	t.Run("refuses limiting an owner", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		admin := d.join(t, enums.RoleAdmin)
		owner := d.join(t, enums.RoleOwner)

		admin.client.PatchJSON("/api/v1/members/"+owner.public(), testkit.JSON{"conversation_access": "assigned"}).
			AssertStatus(422).AssertJSONPath("code", "owner_access_unrestricted")
	})

	t.Run("makes a promoted member see everything and says so in the audit log", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		team := d.team(t, "Billing")
		d.restrict(t, m, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{team}})

		promoted := d.owner.PatchJSON("/api/v1/members/"+m.public(), testkit.JSON{"role": "owner", "keep_owner": true})
		promoted.AssertOK()
		promoted.AssertJSONPath("data.conversation_access", "all")
		promoted.AssertJSONPath("data.access_team_ids.#", 0)
		d.owner.GetJSON("/api/v1/audit-logs?filter[event]=member.access_changed").AssertOK().
			AssertJSONPath("data.#", 2)
	})

	t.Run("checks the list against the workspace and the mode", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		team := d.team(t, "Billing")
		other := open(t)
		foreign := other.team(t, "Elsewhere")
		path := "/api/v1/members/" + m.public()

		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{foreign}}).
			AssertStatus(422).AssertJSONPath("code", "team_not_found")
		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "teams", "access_team_ids": []string{"nope"}}).
			AssertStatus(422).AssertJSONPath("code", "team_not_found")
		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "assigned", "access_team_ids": []string{team}}).
			AssertStatus(422).AssertJSONPath("code", "access_teams_not_used")
		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "all", "access_team_ids": []string{team}}).
			AssertStatus(422).AssertJSONPath("code", "access_teams_not_used")
		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "nobody"}).
			AssertValidationError("conversation_access")

		tooMany := make([]string, 201)
		for i := range tooMany {
			tooMany[i] = team
		}

		d.owner.PatchJSON(path, testkit.JSON{"conversation_access": "teams", "access_team_ids": tooMany}).
			AssertStatus(422).AssertJSONPath("code", "too_many_access_teams")
	})

	t.Run("takes the right permission and ability", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		m := d.join(t, enums.RoleMember)
		other := d.join(t, enums.RoleMember)
		admin := d.join(t, enums.RoleAdmin)
		path := "/api/v1/members/" + other.public()
		body := testkit.JSON{"conversation_access": "assigned"}

		m.client.PatchJSON(path, body).AssertForbidden()
		d.tc.ActingAsToken(admin.member, "members:read").PatchJSON(path, body).AssertForbidden()
		d.tc.ActingAsToken(admin.member, "members:write").PatchJSON(path, body).AssertOK()
	})

	t.Run("another workspace's member is not found", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		other := open(t)
		stranger := other.join(t, enums.RoleMember)

		d.owner.PatchJSON("/api/v1/members/"+stranger.public(), testkit.JSON{"conversation_access": "assigned"}).
			AssertNotFound()
	})
}
