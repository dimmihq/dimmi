package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

/*
trash deletes the test case's workspace as its owner.
*/
func trash(tc *tests.TestCase) *testkit.Response {
	tc.T.Helper()

	body := testkit.JSON{"confirm_slug": tc.Workspace.Workspace.Slug}

	return in(tc, tc.Owner).Send("DELETE", "/api/v1/workspace", body)
}

/*
invite invites the address to the test case's workspace as a member,
as its owner.
*/
func invite(tc *tests.TestCase, email string) *testkit.Response {
	tc.T.Helper()

	return in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{
		"invitations": []testkit.JSON{{"email": email, "role": "member"}},
	})
}

func TestWorkspaceDestroy(t *testing.T) {
	t.Parallel()

	t.Run("moves the workspace to the trash, ending its tokens and pending invitations", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		deleted := record[appevents.WorkspaceDeleted](tc)
		_, second := join(t, tc, enums.RoleOwner)
		_, token := tc.MintToken(second, "members:read")
		invite(tc, "grace@example.com").AssertOK()

		res := trash(tc).AssertOK().
			AssertJSONPath("data.id", workspaceID(tc))
		purgeAfter, err := time.Parse(time.RFC3339, res.String("data.purge_after"))
		require.NoError(t, err)
		assert.True(t, tests.Epoch().Add(30*24*time.Hour).Equal(purgeAfter))
		assert.NotEmpty(t, res.String("data.deleted_at"))

		tc.Table("workspaces").
			AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_by_user_id": tc.Owner.ID})
		tc.Table("personal_access_tokens").AssertCount(0, testkit.Row{"revoked_at": nil})
		tc.Table("invitations").AssertCount(1, testkit.Row{"status": "revoked"})
		in(tc, tc.Owner).GetJSON("/api/v1/workspace").AssertNotFound().AssertJSONPath("code", "workspace_not_found")
		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/api/v1/members").AssertUnauthorized()

		require.Len(t, deleted.all(), 1)
		assert.Equal(t, tc.Owner.ID, deleted.all()[0].DeletedBy)

		mails, sent := tests.Sent[appmail.WorkspaceDeletionScheduledMail](tc, fake)
		require.Len(t, mails, 2)
		assert.Equal(t, tc.Owner.Name, mails[0].DeletedBy)
		assert.Contains(t, sent[0].Message.Subject, tc.Workspace.Workspace.Name)
	})

	t.Run("refuses a confirmation that is not the workspace's address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		in(tc, tc.Owner).Send("DELETE", "/api/v1/workspace", testkit.JSON{"confirm_slug": "something-else"}).
			AssertStatus(422).
			AssertJSONPath("code", "confirm_slug_mismatch").
			AssertValidationError("confirm_slug")
		in(tc, tc.Owner).Send("DELETE", "/api/v1/workspace", testkit.JSON{}).AssertValidationError("confirm_slug")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
	})

	t.Run("is for owners, through their own browser, who confirmed lately", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		body := testkit.JSON{"confirm_slug": tc.Workspace.Workspace.Slug}

		in(tc, admin).Send("DELETE", "/api/v1/workspace", body).AssertForbidden().AssertJSONPath("code", "forbidden")
		tc.ActingAsToken(tc.Workspace.Owner, "settings:write").Send("DELETE", "/api/v1/workspace", body).
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")

		owner := in(tc, tc.Owner)
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)
		owner.Send("DELETE", "/api/v1/workspace", body).
			AssertStatus(423).
			AssertJSONPath("code", "password_confirmation_required")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
	})
}

func TestWorkspaceRestore(t *testing.T) {
	t.Parallel()

	t.Run("restores a workspace in the trash, leaving its tokens and invitations ended", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		restored := record[appevents.WorkspaceRestored](tc)
		_, token := tc.MintToken(tc.Workspace.Owner, "members:read")
		invite(tc, "grace@example.com").AssertOK()
		trash(tc).AssertOK()

		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).
			AssertOK().
			AssertJSONPath("data.deleted_at", nil).
			AssertJSONPath("data.purge_after", nil)

		in(tc, tc.Owner).GetJSON("/api/v1/workspace").AssertOK()
		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/api/v1/members").AssertUnauthorized()
		tc.Table("invitations").AssertCount(1, testkit.Row{"status": "revoked"})
		require.Len(t, restored.all(), 1)

		mail, _ := tests.LastSent[appmail.WorkspaceRestoredMail](tc, fake)
		assert.Equal(t, tc.Owner.Name, mail.RestoredBy)
	})

	t.Run("answers a workspace not in the trash as it is", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		restored := record[appevents.WorkspaceRestored](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).
			AssertOK().
			AssertJSONPath("data.name", tc.Workspace.Workspace.Name)
		assert.Empty(t, restored.all())
	})

	t.Run("does not find a workspace in the trash for anyone but its owners", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		stranger := tc.CreateUser()
		trash(tc).AssertOK()
		body := testkit.JSON{"trashed": false}

		for _, client := range []*testkit.Client{tc.ActingAs(admin), tc.ActingAs(stranger)} {
			client.PatchJSON("/api/v1/workspaces/"+workspaceID(tc), body).
				AssertNotFound().
				AssertJSONPath("code", "workspace_not_found")
		}

		in(tc, tc.Owner).PatchJSON("/api/v1/workspaces/ws_nope", body).AssertNotFound()
		tc.ActingAsToken(tc.Workspace.Owner).PatchJSON("/api/v1/workspaces/"+workspaceID(tc), body).
			AssertForbidden()
	})

	t.Run("takes only trashed false and nothing else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		path := "/api/v1/workspaces/" + workspaceID(tc)

		owner.PatchJSON(path, testkit.JSON{"trashed": false, "name": "Renamed"}).AssertValidationError("name")
		owner.PatchJSON(path, testkit.JSON{"trashed": true}).AssertValidationError("trashed")
		owner.PatchJSON(path, testkit.JSON{}).AssertValidationError("trashed")
	})

	t.Run("cannot bring back a workspace whose purge began or whose time ran out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		trash(tc).AssertOK()
		path := "/api/v1/workspaces/" + workspaceID(tc)

		exec(t, tc, `UPDATE workspaces SET settings = settings || '{"purge_started_at": "2026-11-01T00:00:00Z"}'`)
		in(tc, tc.Owner).PatchJSON(path, testkit.JSON{"trashed": false}).
			AssertStatus(410).
			AssertJSONPath("code", "workspace_purging")

		exec(t, tc, `UPDATE workspaces SET settings = settings - 'purge_started_at', purge_after = $1`, tests.Epoch())
		in(tc, tc.Owner).PatchJSON(path, testkit.JSON{"trashed": false}).AssertStatus(410)
	})

	t.Run("asks an owner who signed in a while ago to confirm first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		trash(tc).AssertOK()
		owner := in(tc, tc.Owner)
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)

		owner.PatchJSON("/api/v1/workspaces/"+workspaceID(tc), testkit.JSON{"trashed": false}).
			AssertStatus(423)
	})
}
