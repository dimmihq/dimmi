package workspaces_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestAuditLogOfAccessTeamsAndTrash(t *testing.T) {
	t.Parallel()

	t.Run("records a token minted and revoked under its name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		id := owner.PostJSON(patPath, testkit.JSON{"name": "CI deploy", "abilities": []string{"members:read"}}).
			AssertCreated().String("data.id")
		owner.Delete(patPath + "/" + id).AssertNoContent()

		tc.Table("audit_logs").
			AssertHas(testkit.Row{"event": "token.created", "subject_type": "token", "subject_label": "CI deploy"}).
			AssertHas(testkit.Row{"event": "token.revoked", "subject_type": "token", "subject_label": "CI deploy"})
	})

	t.Run("records an invitation sent and withdrawn under its address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		id := owner.PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{{
			"email": "dave@example.com", "role": "member",
		}}}).AssertOK().String("data.0.invitation.id")
		owner.Delete("/api/v1/invitations/" + id).AssertNoContent()

		tc.Table("audit_logs").
			AssertHas(testkit.Row{"event": "invitation.created", "subject_label": "dave@example.com"}).
			AssertHas(testkit.Row{"event": "invitation.revoked", "subject_label": "dave@example.com"})
	})

	t.Run("records a team's life and who joined and left it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, member := join(t, tc, enums.RoleMember)

		team := owner.PostJSON("/api/v1/teams", testkit.JSON{"name": "Support"}).AssertCreated().String("data.id")
		owner.PatchJSON("/api/v1/teams/"+team, testkit.JSON{"name": "Help"}).AssertOK()
		owner.PostJSON("/api/v1/teams/"+team+"/members", testkit.JSON{"member_ids": []string{memberID(member)}}).
			AssertOK()
		owner.Delete("/api/v1/teams/" + team + "/members/" + memberID(member)).AssertNoContent()
		owner.Delete("/api/v1/teams/" + team).AssertNoContent()

		tc.Table("audit_logs").
			AssertHas(testkit.Row{"event": "team.created", "subject_label": "Support"}).
			AssertHas(testkit.Row{"event": "team.updated", "subject_label": "Help"}).
			AssertHas(testkit.Row{"event": "team.member_added", "subject_type": "member", "subject_id": member.ID}).
			AssertHas(testkit.Row{"event": "team.member_removed", "subject_type": "member", "subject_id": member.ID}).
			AssertHas(testkit.Row{"event": "team.deleted", "subject_label": "Help"})
	})

	t.Run("records the workspace moved to the trash and brought back by its owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		trash(tc).AssertOK()
		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).AssertOK()

		tc.Table("audit_logs").
			AssertHas(testkit.Row{"event": "workspace.deleted", "actor_id": tc.Workspace.Owner.ID}).
			AssertHas(testkit.Row{"event": "workspace.restored", "actor_id": tc.Workspace.Owner.ID})
	})
}
