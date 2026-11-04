package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
accept accepts the invitation the token belongs to, signed in as the
user.
*/
func accept(tc *tests.TestCase, client *testkit.Client, token string) *testkit.Response {
	tc.T.Helper()

	return client.PostJSON("/api/v1/user/memberships", testkit.JSON{"invitation_token": token})
}

/*
preview shows the invitation the token belongs to, to someone signed
out.
*/
func preview(tc *tests.TestCase, token string) *testkit.Response {
	tc.T.Helper()

	return tc.Client().WithHeader("Dimmi-Invitation-Token", token).GetJSON("/api/v1/invitations/preview")
}

func TestInvitationPreview(t *testing.T) {
	t.Parallel()

	t.Run("shows the holder of a link what it invites to", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		token := mailedToken(tc, fake, "grace@example.com", "admin")

		preview(tc, token).AssertOK().
			AssertHeader("Cache-Control", "no-store").
			AssertJSONPath("data.object", "invitation_preview").
			AssertJSONPath("data.workspace.id", workspaceID(tc)).
			AssertJSONPath("data.workspace.name", tc.Workspace.Workspace.Name).
			AssertJSONPath("data.inviter.name", tc.Owner.Name).
			AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.email_masked", "g***@example.com").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.has_account", false)

		tc.CreateUser(func(u models.User) models.User { u.Email = "grace@example.com"; return u })
		preview(tc, token).AssertOK().AssertJSONPath("data.has_account", true)

		tc.Clock.Travel(8 * 24 * time.Hour)
		preview(tc, token).AssertOK().AssertJSONPath("data.status", "expired")
	})

	t.Run("does not find a link of no invitation, or of a workspace in the trash", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		token := mailedToken(tc, fake, "grace@example.com", "member")

		preview(tc, "made-up").AssertNotFound().AssertJSONPath("code", "not_found")
		tc.Client().GetJSON("/api/v1/invitations/preview").AssertNotFound()

		trash(tc).AssertOK()
		preview(tc, token).AssertNotFound()
	})
}

func TestInvitationAccept(t *testing.T) {
	t.Parallel()

	t.Run("makes the invited user a member, verified, in the invitation's role and teams", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		joined := record[appevents.MemberJoined](tc)
		accepted := record[appevents.InvitationAccepted](tc)
		grace := tc.CreateUser(factories.Unverified)
		team := ids.New()
		exec(t, tc, `INSERT INTO teams (id, workspace_id, name) VALUES ($1, $2, 'Support')`,
			team, tc.Workspace.Workspace.ID)
		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{{
			"email": grace.Email, "role": "admin", "team_ids": []string{models.PublicID(models.PrefixTeam, team)},
		}}}).AssertOK()
		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		token := tc.Fragment(sent.AcceptURL).Get("token")

		res := accept(tc, tc.ActingAs(grace), token).AssertCreated().
			AssertJSONPath("data.object", "membership").
			AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.joined_via", "invitation").
			AssertJSONPath("data.workspace.id", workspaceID(tc))

		member := res.String("data.id")
		tc.Table("users").AssertCount(0, testkit.Row{"id": grace.ID, "email_verified_at": nil})
		tc.Table("team_members").AssertCount(1, testkit.Row{"team_id": team})
		tc.Table("invitations").AssertHas(testkit.Row{"status": "accepted", "accepted_by_user_id": grace.ID})
		require.Len(t, joined.all(), 1)
		require.Len(t, accepted.all(), 1)
		assert.Equal(t, member, models.PublicID(models.PrefixMember, accepted.all()[0].MemberID))

		mail, message := tests.LastSent[appmail.InvitationAcceptedMail](tc, fake)
		assert.Equal(t, grace.Name, mail.Member)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(message))

		accept(tc, tc.ActingAs(grace), token).AssertStatus(409).AssertJSONPath("code", "invitation_already_accepted")
		in(tc, grace).GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("belongs to the invited address only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		token := mailedToken(tc, fake, "grace@example.com", "member")
		stranger := tc.CreateUser()

		accept(tc, tc.ActingAs(stranger), token).AssertForbidden().AssertJSONPath("code", "invitation_email_mismatch")
		accept(tc, tc.Client(), token).AssertUnauthorized()
		accept(tc, tc.ActingAs(stranger), "made-up").AssertNotFound()
		tc.ActingAs(stranger).PostJSON("/api/v1/user/memberships", testkit.JSON{
			"invitation_token": token, "workspace_id": workspaceID(tc),
		}).AssertValidationError("workspace_id")
		tc.ActingAs(stranger).PostJSON("/api/v1/user/memberships", testkit.JSON{}).
			AssertValidationError("invitation_token")
		tc.Table("invitations").AssertHas(testkit.Row{"status": "pending"})
	})

	t.Run("refuses an invitation that expired or was withdrawn", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		grace := tc.CreateUser()
		token := mailedToken(tc, fake, grace.Email, "member")

		exec(t, tc, `UPDATE invitations SET expires_at = $1`, tests.Epoch().Add(-time.Minute))
		accept(tc, tc.ActingAs(grace), token).AssertStatus(410).AssertJSONPath("code", "invitation_expired")

		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertNoContent()
		accept(tc, tc.ActingAs(grace), token).AssertStatus(410).AssertJSONPath("code", "invitation_revoked")
		tc.Table("workspace_members").AssertCount(0, testkit.Row{"user_id": grace.ID})
	})

	t.Run("brings a deactivated member back with their old membership", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		reactivated := record[appevents.MemberReactivated](tc)
		grace, old := join(t, tc, enums.RoleAdmin)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, old))
		token := mailedToken(tc, fake, grace.Email, "member")

		accept(tc, tc.ActingAs(grace), token).AssertCreated().
			AssertJSONPath("data.id", memberID(old)).
			AssertJSONPath("data.role", "member")
		require.Len(t, reactivated.all(), 1)
	})

	t.Run("refuses someone already active in the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		grace := tc.CreateUser()
		token := mailedToken(tc, fake, grace.Email, "member")
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: grace, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		accept(tc, tc.ActingAs(grace), token).AssertStatus(409).AssertJSONPath("code", "already_member")
	})

	t.Run("does not tell an inviter who left the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		grace := tc.CreateUser()
		in(tc, admin).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": grace.Email, "role": "member"}},
		}).AssertOK()
		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, adminMember))

		accept(tc, tc.ActingAs(grace), tc.Fragment(sent.AcceptURL).Get("token")).AssertCreated()
		mails, _ := tests.Sent[appmail.InvitationAcceptedMail](tc, fake)
		assert.Empty(t, mails)
	})

	t.Run("is for the user themselves, not their tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		accept(tc, tc.ActingAsToken(tc.Workspace.Owner), "anything").AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})
}

func TestSignUpWithInvitation(t *testing.T) {
	t.Parallel()

	t.Run("signs an invited person up into the workspace, verified, even where signup is closed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		fake := tc.Mail()
		token := mailedToken(tc, fake, "grace@example.com", "admin")

		tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Grace Hopper", "email": "grace@example.com", "password": "analytical-engine-1843",
		}).AssertForbidden().AssertJSONPath("code", "signup_closed")

		tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Grace Hopper", "email": "Grace@Example.com", "password": "analytical-engine-1843",
			"invitation_token": token,
		}).AssertCreated().
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.workspace.id", workspaceID(tc)).
			AssertJSONPath("data.member.role", "admin")

		tc.Table("users").AssertHas(testkit.Row{"email": "Grace@Example.com", "created_via": "invitation"})
		tc.Table("invitations").AssertHas(testkit.Row{"status": "accepted"})
		verifications, _ := tests.Sent[appmail.VerifyEmailMail](tc, fake)
		assert.Empty(t, verifications)
	})

	t.Run("refuses another address, and creates nothing for a used invitation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		token := mailedToken(tc, fake, "grace@example.com", "member")
		body := testkit.JSON{
			"name": "Ada", "email": "ada@example.com", "password": "analytical-engine-1843", "invitation_token": token,
		}

		tc.Client().PostJSON("/api/v1/users", body).AssertForbidden().
			AssertJSONPath("code", "invitation_email_mismatch")

		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertNoContent()
		body["email"] = "grace@example.com"
		tc.Client().PostJSON("/api/v1/users", body).AssertStatus(410)
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("mails a sign-up link to an invited address where signup is closed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		fake := tc.Mail()

		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": "grace@example.com"}).AssertStatus(202)
		links, _ := tests.Sent[appmail.MagicLinkMail](tc, fake)
		assert.Empty(t, links)

		mailedToken(tc, fake, "grace@example.com", "member")
		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": "grace@example.com"}).AssertStatus(202)
		links, _ = tests.Sent[appmail.MagicLinkMail](tc, fake)
		assert.Len(t, links, 1)
	})
}
