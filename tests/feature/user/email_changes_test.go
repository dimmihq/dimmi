package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const newAddress = "ada.new@example.com"

/*
askToMove starts a move of the owner's account and returns the link's id
and token, read from the mail.
*/
func askToMove(t *testing.T, tc *tests.TestCase, owner *testkit.Client, email string) (string, string) {
	t.Helper()

	fake := tc.Mail()
	owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": email}).AssertCreated()
	sent, _ := tests.LastSent[appmail.EmailChangeVerificationMail](tc, fake)
	link := tc.Fragment(sent.URL)

	return link.Get("id"), link.Get("token")
}

func confirmMove(client *testkit.Client, id, token string) *testkit.Response {
	return client.PatchJSON("/api/v1/email-changes/"+id, testkit.JSON{"token": token})
}

func TestEmailChangeStore(t *testing.T) {
	t.Parallel()

	t.Run("mails the link to the new address and warns the old one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		tc.ActingAsOwner().PostJSON("/api/v1/email-changes", testkit.JSON{"email": "Ada.New@Example.com"}).
			AssertCreated().
			AssertJSONPath("data.object", "email_change").
			AssertJSONPath("data.status", "pending").
			AssertJSONPath("data.email", "a***@example.com")

		verification, toNew := tests.LastSent[appmail.EmailChangeVerificationMail](tc, fake)
		assert.Equal(t, []string{newAddress}, tests.SentTo(toNew))
		assert.Contains(t, verification.URL, "/email/confirm#id=ott_")

		warning, toOld := tests.LastSent[appmail.EmailChangeRequestedMail](tc, fake)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(toOld))
		assert.Equal(t, "a***@example.com", warning.MaskedEmail)
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "email": tc.Owner.Email})
		tc.Table("security_events").AssertHas(testkit.Row{
			"user_id": tc.Owner.ID, "type": auth.EventEmailChangeRequested,
		})
	})

	t.Run("refuses an address another account holds, and the account's own", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tc.CreateUser()
		owner := tc.ActingAsOwner()

		owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": other.Email}).
			AssertValidationError("email").AssertJSONPath("code", "email_taken")
		owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": tc.Owner.Email}).
			AssertValidationError("email").AssertJSONPath("code", "email_unchanged")
		owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": "not an address"}).
			AssertValidationError("email")
	})

	t.Run("needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		tc.Clock.Travel(confirmationWindow + time.Second)

		owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": newAddress}).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("allows five requests an hour", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		for range 5 {
			owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": newAddress}).AssertCreated()
		}

		owner.PostJSON("/api/v1/email-changes", testkit.JSON{"email": newAddress}).
			AssertStatus(fhttp.StatusTooManyRequests)
	})

	t.Run("a token cannot start a move", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner).PostJSON("/api/v1/email-changes", testkit.JSON{"email": newAddress}).
			AssertForbidden()
	})
}

func TestEmailChangeUpdate(t *testing.T) {
	t.Parallel()

	t.Run("a person changes their email from any device and the old address hears", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)

		confirmMove(tc.Client(), id, token).AssertOK().
			AssertJSONPath("data.object", "email_change").
			AssertJSONPath("data.status", "confirmed").
			AssertJSONPath("data.email", newAddress)

		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "email": newAddress})
		_, told := tests.LastSent[appmail.EmailChangedMail](tc, fake)
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(told))
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": auth.EventEmailChanged})

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": newAddress, "password": factories.Password,
		}).AssertCreated()
	})

	t.Run("a browser signed in to the account moves to a new session and stays signed in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		id, token := askToMove(t, tc, owner, newAddress)

		res := confirmMove(owner, id, token).AssertOK()

		tc.Client().WithCookie(res.Cookie(tc.App.Session.CookieName())).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.email", newAddress)
	})

	t.Run("second redemption is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)

		confirmMove(tc.Client(), id, token).AssertOK()
		confirmMove(tc.Client(), id, token).
			AssertValidationError("token").AssertJSONPath("code", "invalid_email_change_token")
	})

	t.Run("a wrong token, an unknown id and an expired link fail alike", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)

		confirmMove(tc.Client(), id, "wrong").AssertValidationError("token").
			AssertJSONPath("code", "invalid_email_change_token")
		confirmMove(tc.Client(), "ott_nothing", token).AssertValidationError("token").
			AssertJSONPath("code", "invalid_email_change_token")

		tc.Clock.Travel(25 * time.Hour)
		confirmMove(tc.Client(), id, token).AssertValidationError("token").
			AssertJSONPath("code", "invalid_email_change_token")
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "email": tc.Owner.Email})
	})

	t.Run("five wrong tokens burn the link", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)

		for range 5 {
			confirmMove(tc.Client(), id, "wrong").AssertValidationError("token")
		}

		confirmMove(tc.Client(), id, token).AssertValidationError("token")
	})

	t.Run("a new request ends the one before it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		firstID, firstToken := askToMove(t, tc, owner, newAddress)
		secondID, secondToken := askToMove(t, tc, owner, "second@example.com")

		confirmMove(tc.Client(), firstID, firstToken).AssertValidationError("token")
		confirmMove(tc.Client(), secondID, secondToken).AssertOK().AssertJSONPath("data.email", "second@example.com")
	})

	t.Run("an address someone registered meanwhile drops the change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)
		tc.CreateUser(func(u models.User) models.User {
			u.Email = newAddress

			return u
		})

		confirmMove(tc.Client(), id, token).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "email_taken")
		confirmMove(tc.Client(), id, token).AssertValidationError("token")
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "email": tc.Owner.Email})
	})

	t.Run("links mailed to the old address stop working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		sent, _ := tests.LastSent[appmail.ResetPasswordMail](tc, fake)
		reset := tc.Fragment(sent.URL)
		id, token := askToMove(t, tc, tc.ActingAsOwner(), newAddress)

		confirmMove(tc.Client(), id, token).AssertOK()

		tc.Client().PatchJSON("/api/v1/password-resets/"+reset.Get("id"), testkit.JSON{
			"token": reset.Get("token"), "password": nextPhrase,
		}).AssertValidationError("token")
	})

	t.Run("a second factor keeps working after the move", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		browser := tc.ActingAs(user)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		id, token := askToMove(t, tc, browser, newAddress)

		confirmMove(tc.Client(), id, token).AssertOK()

		tc.Table("user_auth_factors").AssertCount(1, testkit.Row{"user_id": user.ID, "type": "totp"})
	})

	t.Run("a pending invitation to the new address is accepted once the account moved there", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		tc.ActingAsOwner().PostJSON("/api/v1/invitations", testkit.JSON{
			"email": newAddress, "role": "member",
		}).AssertOK()
		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		invite := tc.Fragment(sent.AcceptURL).Get("token")
		user := tc.CreateUser()
		browser := tc.ActingAs(user)
		id, token := askToMove(t, tc, browser, newAddress)

		moved := tc.Client().WithCookie(confirmMove(browser, id, token).AssertOK().Cookie(tc.App.Session.CookieName()))

		moved.PostJSON("/api/v1/user/memberships", testkit.JSON{"invitation_token": invite}).AssertOK()
	})
}
