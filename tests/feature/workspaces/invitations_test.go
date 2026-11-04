package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
mailedToken invites the address as the role and returns the token its
invitation mail carries.
*/
func mailedToken(tc *tests.TestCase, fake *mail.Fake, email, role string) string {
	tc.T.Helper()

	in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{
		"invitations": []testkit.JSON{{"email": email, "role": role}},
	}).AssertOK()

	sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)

	return tc.Fragment(sent.AcceptURL).Get("token")
}

/*
mailServer configures a mail transport, so the server mails
invitations instead of handing their links back.
*/
func mailServer() tests.Option {
	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=smtp", "DIMMI_MAIL__MAILERS__SYSTEM__HOST=localhost",
		"DIMMI_MAIL__MAILERS__SYSTEM__PORT=11025",
	)
}

/*
invitationID returns the public id of the newest invitation.
*/
func invitationID(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	var id models.Invitation

	row := tc.Operator().QueryRow(t.Context(), `SELECT id FROM invitations ORDER BY created_at DESC, id DESC LIMIT 1`)
	require.NoError(t, row.Scan(&id.ID))

	return models.PublicID(models.PrefixInvitation, id.ID)
}

func TestInvitationsStore(t *testing.T) {
	t.Parallel()

	t.Run("answers for every address of a batch", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, mailServer())
		fake := tc.Mail()
		created := record[appevents.InvitationCreated](tc)
		member, _ := join(t, tc, enums.RoleMember)
		gone, left := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, left))

		res := in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{
			{"email": "  Grace@ACME.com ", "role": "admin"},
			{"email": member.Email, "role": "member"},
			{"email": gone.Email, "role": "member"},
			{"email": "not-an-email", "role": "member"},
		}}).AssertOK()

		res.AssertJSONPath("data.0.result", "created").
			AssertJSONPath("data.0.invitation.email", "Grace@acme.com").
			AssertJSONPath("data.0.invitation.role", "admin").
			AssertJSONPath("data.0.invitation.status", "pending").
			AssertJSONPath("data.0.invitation.invited_by.name", tc.Owner.Name).
			AssertJSONPath("data.1.result", "already_member").
			AssertJSONPath("data.1.invitation", nil).
			AssertJSONPath("data.2.result", "reactivation").
			AssertJSONPath("data.3.result", "invalid").
			AssertJSONPath("data.3.error.code", "invalid_email")

		assert.Contains(t, res.String("data.0.invitation.accept_url"), "/invitation#token=")
		assert.Len(t, created.all(), 2)
		mails, sent := tests.Sent[appmail.InvitationMail](tc, fake)
		require.Len(t, mails, 2)

		for i, mail := range mails {
			if mail.Role != "admin" {
				continue
			}

			assert.Equal(t, []string{"Grace@acme.com"}, tests.SentTo(sent[i]))
			assert.Contains(t, sent[i].Message.Subject, "invited you")
			assert.Equal(t, tc.Owner.Name, mail.Inviter)
			assert.NotEmpty(t, tc.Fragment(mail.AcceptURL).Get("token"))
		}
	})

	t.Run("sends a pending invitation again with a fresh link instead of a second one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		first := mailedToken(tc, fake, "grace@example.com", "member")

		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "GRACE@example.com", "role": "admin"}},
		}).AssertOK().
			AssertJSONPath("data.0.result", "resent").
			AssertJSONPath("data.0.invitation.role", "admin").
			AssertJSONPath("data.0.invitation.send_count", 2)

		tc.Table("invitations").AssertCount(1)
		tc.Client().WithHeader("Dimmi-Invitation-Token", first).GetJSON("/api/v1/invitations/preview").
			AssertNotFound()
	})

	t.Run("hands the link to the admin whether or not the server can mail it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := invite(tc, "grace@example.com").AssertOK()
		assert.Contains(t, res.String("data.0.invitation.accept_url"), "/invitation#token=")
	})

	t.Run("refuses members, tokens without the ability, bad roles and big batches", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		one := []testkit.JSON{{"email": "grace@example.com", "role": "member"}}

		in(tc, member).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": one}).AssertForbidden()
		tc.ActingAsToken(tc.Workspace.Owner, "members:read").
			PostJSON("/api/v1/invitations", testkit.JSON{"invitations": one}).
			AssertForbidden().
			AssertJSONPath("code", "missing_ability")
		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "grace@example.com", "role": "owner"}},
		}).AssertValidationError("invitations.0.role")
		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{}}).
			AssertValidationError("invitations")

		many := make([]testkit.JSON, 51)
		for i := range many {
			many[i] = testkit.JSON{"email": "grace@example.com", "role": "member"}
		}

		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": many}).
			AssertValidationError("invitations")
		tc.Table("invitations").AssertCount(0)
	})

	t.Run("stops a workspace at its daily cap", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_INVITATIONS__MAX_PER_DAY=2"))

		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{
			{"email": "a@example.com", "role": "member"}, {"email": "b@example.com", "role": "member"},
		}}).AssertOK()
		invite(tc, "c@example.com").
			AssertStatus(429).
			AssertJSONPath("code", "invitation_limit_reached")
	})

	t.Run("needs a verified email where the install asks for one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__VERIFICATION__REQUIRED=true"))
		admin, _ := join(t, tc, enums.RoleAdmin, factories.Unverified)

		in(tc, admin).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "grace@example.com", "role": "member"}},
		}).AssertForbidden().AssertJSONPath("code", "email_unverified")
	})
}

func TestInvitationsIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists pending invitations, newest first, and others when asked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		invite(tc, "ada@example.com").AssertOK()
		tc.Clock.Travel(time.Minute)
		invite(tc, "grace@example.com").AssertOK()
		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertNoContent()

		in(tc, tc.Owner).GetJSON("/api/v1/invitations").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.email", "ada@example.com").
			AssertJSONPath("data.0.accept_url", nil)
		in(tc, tc.Owner).GetJSON("/api/v1/invitations?filter[status]=revoked,pending&limit=1").AssertOK().
			AssertJSONPath("data.0.email", "grace@example.com").
			AssertJSONPath("data.0.status", "revoked")
		in(tc, tc.Owner).GetJSON("/api/v1/invitations?filter[status]=lost").AssertStatus(400)
	})

	t.Run("lists an invitation past its expiry as expired", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		invite(tc, "ada@example.com").AssertOK()
		exec(t, tc, `UPDATE invitations SET expires_at = $1`, tests.Epoch().Add(-time.Minute))

		in(tc, tc.Owner).GetJSON("/api/v1/invitations").AssertOK().AssertJSONPath("data.#", 0)
		in(tc, tc.Owner).GetJSON("/api/v1/invitations?filter[status]=expired").AssertOK().
			AssertJSONPath("data.0.status", "expired")
	})

	t.Run("is for admins and owners", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)

		in(tc, member).GetJSON("/api/v1/invitations").AssertForbidden()
	})
}

func TestInvitationDeliveries(t *testing.T) {
	t.Parallel()

	t.Run("sends an invitation again with a fresh link and a fresh week", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		first := mailedToken(tc, fake, "grace@example.com", "member")
		exec(t, tc, `UPDATE invitations SET expires_at = $1`, tests.Epoch().Add(-time.Minute))
		tc.Clock.Travel(time.Hour)

		res := in(tc, tc.Owner).PostJSON("/api/v1/invitations/"+invitationID(t, tc)+"/deliveries", nil).
			AssertCreated().
			AssertJSONPath("data.object", "invitation_delivery").
			AssertJSONPath("data.invitation.status", "pending").
			AssertJSONPath("data.invitation.send_count", 2)
		expires, err := time.Parse(time.RFC3339, res.String("data.invitation.expires_at"))
		require.NoError(t, err)
		assert.True(t, tests.Epoch().Add(time.Hour+7*24*time.Hour).Equal(expires))

		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		fresh := tc.Fragment(sent.AcceptURL).Get("token")
		assert.NotEqual(t, first, fresh)
		tc.Client().WithHeader("Dimmi-Invitation-Token", fresh).GetJSON("/api/v1/invitations/preview").AssertOK()
	})

	t.Run("copy link rotates the token and the emailed link stops working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		mailed := mailedToken(tc, fake, "grace@example.com", "member")

		res := in(tc, tc.Owner).PostJSON("/api/v1/invitations/"+invitationID(t, tc)+"/deliveries",
			testkit.JSON{"channel": "link"}).AssertCreated().
			AssertJSONPath("data.invitation.send_count", 2)
		copied := tc.Fragment(res.String("data.invitation.accept_url")).Get("token")

		mails, _ := tests.Sent[appmail.InvitationMail](tc, fake)
		assert.Len(t, mails, 1)
		tc.Client().WithHeader("Dimmi-Invitation-Token", mailed).GetJSON("/api/v1/invitations/preview").
			AssertNotFound()
		tc.Client().WithHeader("Dimmi-Invitation-Token", copied).GetJSON("/api/v1/invitations/preview").AssertOK()
		in(tc, tc.Owner).PostJSON("/api/v1/invitations/"+invitationID(t, tc)+"/deliveries",
			testkit.JSON{"channel": "fax"}).AssertValidationError("channel")
	})

	t.Run("sends one invitation at most three times a day", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		invite(tc, "grace@example.com").AssertOK()
		path := "/api/v1/invitations/" + invitationID(t, tc) + "/deliveries"

		for range 3 {
			in(tc, tc.Owner).PostJSON(path, nil).AssertCreated()
		}

		in(tc, tc.Owner).PostJSON(path, nil).AssertStatus(429).AssertJSONPath("code", "too_many_attempts")
	})

	t.Run("refuses answered invitations and ids of other workspaces", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		invite(tc, "grace@example.com").AssertOK()
		id := invitationID(t, tc)
		in(tc, tc.Owner).Delete("/api/v1/invitations/" + id).AssertNoContent()

		in(tc, tc.Owner).PostJSON("/api/v1/invitations/"+id+"/deliveries", nil).
			AssertStatus(422).
			AssertJSONPath("code", "invalid_transition").
			AssertJSONPath("from", "revoked")

		other := tests.New(t)
		in(other, other.Owner).PostJSON("/api/v1/invitations/"+id+"/deliveries", nil).AssertNotFound()
		in(other, other.Owner).Delete("/api/v1/invitations/" + id).AssertNotFound()
		in(tc, tc.Owner).Delete("/api/v1/invitations/inv_nope").AssertNotFound()
	})
}

func TestInvitationsDestroy(t *testing.T) {
	t.Parallel()

	t.Run("withdraws a pending invitation, whose link then says so", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		revoked := record[appevents.InvitationRevoked](tc)
		token := mailedToken(tc, fake, "grace@example.com", "member")

		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertNoContent()
		in(tc, tc.Owner).Delete("/api/v1/invitations/" + invitationID(t, tc)).AssertStatus(422)

		require.Len(t, revoked.all(), 1)
		tc.Client().WithHeader("Dimmi-Invitation-Token", token).GetJSON("/api/v1/invitations/preview").AssertOK().
			AssertJSONPath("data.status", "revoked")
	})
}
