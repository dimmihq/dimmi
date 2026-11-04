package workspaces_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
refuse makes every listener of the event refuse it inside its
transaction, the way a failing audit write would.
*/
func refuse[E any](tc *tests.TestCase) {
	events.Listen(tc.App.Events, events.InTx, func(context.Context, E) error { return errors.New("refused") })
}

/*
invitee returns an account for grace@example.com and the token of the
invitation the owner mailed her.
*/
func invitee(tc *tests.TestCase) (models.User, string) {
	tc.T.Helper()

	grace := tc.CreateUser(func(u models.User) models.User { u.Email = "grace@example.com"; return u })

	return grace, mailedToken(tc, tc.Mail(), grace.Email, "member")
}

/*
inviteGrace invites grace@example.com as a member through the browser.
*/
func inviteGrace(browser *testkit.Client) *testkit.Response {
	return browser.PostJSON("/api/v1/invitations", testkit.JSON{
		"invitations": []testkit.JSON{{"email": "grace@example.com", "role": "member"}},
	})
}

/*
acceptAs accepts the invitation as the user through the action.
*/
func acceptAs(tc *tests.TestCase, user models.User, token string) error {
	_, err := tc.App.Actions.AcceptInvitation.Handle(tc.T.Context(), invitations.AcceptInvitationInput{
		User: user, Token: token,
	})

	return err
}

func TestInvitationStorageFailures(t *testing.T) {
	t.Parallel()

	t.Run("invites nobody when the invitation cannot be made", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]struct {
			breaks func(tc *tests.TestCase)
			status int
		}{
			"teams":      {func(tc *tests.TestCase) { tc.BreakTable("teams") }, 500},
			"storing":    {func(tc *tests.TestCase) { tc.FailWrites("invitations", "INSERT") }, 500},
			"mailing":    {func(tc *tests.TestCase) { tc.FailWrites("river_job", "INSERT") }, 500},
			"listener":   {refuse[appevents.InvitationCreated], 500},
			"rate limit": {func(tc *tests.TestCase) { tc.BreakValkey() }, 503},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t, tests.WithEnv("DIMMI_INVITATIONS__MAX_PER_DAY=50"))
				owner := in(tc, tc.Owner)
				out.breaks(tc)

				inviteGrace(owner).AssertStatus(out.status)
				if name != "teams" {
					tc.Table("invitations").AssertCount(0)
				}
			})
		}
	})

	t.Run("keeps the old link when a pending invitation cannot be sent again", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		inviteGrace(owner).AssertOK()
		id := invitationID(t, tc)
		tc.FailWrites("invitations", "UPDATE")

		inviteGrace(owner).AssertStatus(500)
		owner.PostJSON("/api/v1/invitations/"+id+"/deliveries", nil).AssertStatus(500)
		owner.Delete("/api/v1/invitations/" + id).AssertStatus(500)
		tc.Table("invitations").AssertHas(testkit.Row{"status": "pending", "send_count": 1})
	})

	t.Run("refuses a member's address when the members cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.Workspace.Owner
		membership := authz.NewMembership(tc.Workspace.Workspace, owner, []string{"members:write"})
		tc.BreakTable("workspace_members")

		_, err := tc.App.Actions.CreateInvitations.Handle(t.Context(), invitations.CreateInvitationsInput{
			Membership: membership, Inviter: tc.Owner,
			Invitees: []invitations.Invitee{{Email: "grace@example.com", Role: enums.RoleMember}},
		})

		require.Error(t, err)
	})

	t.Run("sends nothing again when the invitation cannot be read, counted or mailed", func(t *testing.T) {
		t.Parallel()

		for name, out := range map[string]struct {
			breaks func(tc *tests.TestCase)
			status int
		}{
			"reading":    {func(tc *tests.TestCase) { tc.BreakTable("invitations") }, 500},
			"mailing":    {func(tc *tests.TestCase) { tc.FailWrites("river_job", "INSERT") }, 500},
			"rate limit": {func(tc *tests.TestCase) { tc.BreakValkey() }, 503},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				owner := in(tc, tc.Owner)
				inviteGrace(owner).AssertOK()
				id := invitationID(t, tc)
				out.breaks(tc)

				owner.PostJSON("/api/v1/invitations/"+id+"/deliveries", nil).AssertStatus(out.status)
			})
		}
	})

	t.Run("withdraws nothing when the withdrawal is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		invite(tc, "grace@example.com").AssertOK()
		refuse[appevents.InvitationRevoked](tc)

		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertStatus(500)
		tc.Table("invitations").AssertHas(testkit.Row{"status": "pending"})
	})

	t.Run("lists nothing when invitations cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.BreakTable("invitations")

		owner.GetJSON("/api/v1/invitations").AssertStatus(500)
	})
}

func TestInvitationAcceptFailures(t *testing.T) {
	t.Parallel()

	t.Run("joins nobody when the membership cannot be made", func(t *testing.T) {
		t.Parallel()

		for name, breaks := range map[string]func(tc *tests.TestCase){
			"verifying the address": func(tc *tests.TestCase) { tc.FailWrites("users", "UPDATE") },
			"adding the member":     func(tc *tests.TestCase) { tc.FailWrites("workspace_members", "INSERT") },
			"joining teams":         func(tc *tests.TestCase) { tc.FailWrites("team_members", "INSERT") },
			"marking it accepted":   func(tc *tests.TestCase) { tc.FailWrites("invitations", "UPDATE") },
			"telling the inviter":   func(tc *tests.TestCase) { tc.FailWrites("river_job", "INSERT") },
			"listener":              refuse[appevents.InvitationAccepted],
			"invitations":           func(tc *tests.TestCase) { tc.BreakTable("invitations") },
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				grace, token := invitee(tc)
				breaks(tc)

				require.Error(t, acceptAs(tc, grace, token))
				if name != "invitations" {
					tc.Table("workspace_members").AssertCount(0, testkit.Row{"user_id": grace.ID})
				}
			})
		}
	})

	t.Run("keeps a deactivated member away when they cannot be brought back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		grace, old := join(t, tc, enums.RoleAdmin)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, old))
		token := mailedToken(tc, tc.Mail(), grace.Email, "member")
		tc.FailWrites("workspace_members", "UPDATE")

		require.Error(t, acceptAs(tc, grace, token))
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": old.ID, "status": "deactivated"})
	})

	t.Run("accepts an invitation whose sender is gone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		grace, old := join(t, tc, enums.RoleAdmin)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, old))
		token := mailedToken(tc, tc.Mail(), grace.Email, "member")
		exec(t, tc, `UPDATE invitations SET invited_by_member_id = NULL`)

		require.NoError(t, acceptAs(tc, grace, token))
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": old.ID, "status": "active"})
	})

	t.Run("previews nothing when the invitation or its address cannot be read", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"invitations", "users"} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				_, token := invitee(tc)
				tc.BreakTable(table)

				_, err := tc.App.Actions.PreviewInvitation.Handle(t.Context(), token)

				require.Error(t, err)
			})
		}
	})
}
