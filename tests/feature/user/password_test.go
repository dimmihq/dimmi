package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const nextPhrase = "a-fresh-long-passphrase-1843"

func TestUserPasswordUpdate(t *testing.T) {
	t.Parallel()

	t.Run("a person changes their password and other sessions end", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		other, here := tc.ActingAsOwner(), tc.ActingAsOwner()

		res := here.PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": nextPhrase,
		}).AssertOK().
			AssertJSONPath("data.object", "password_change").
			AssertJSONPath("data.tokens_remaining", 0)

		other.GetJSON("/api/v1/me").AssertUnauthorized()
		here.WithCookie(res.Cookie(tc.App.Session.CookieName())).GetJSON("/api/v1/me").AssertOK()
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": nextPhrase,
		}).AssertCreated()

		mail, _ := tests.LastSent[appmail.PasswordChangedMail](tc, fake)
		assert.Equal(t, appmail.PasswordWasChanged, mail.Reason)
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": auth.EventPasswordChanged})
	})

	t.Run("access tokens keep working and are counted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		token := tc.ActingAsToken(tc.Workspace.Owner)

		tc.ActingAsOwner().PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": nextPhrase,
		}).AssertOK().AssertJSONPath("data.tokens_remaining", 1)

		token.GetJSON("/api/v1/me").AssertOK()
	})

	t.Run("a wrong or missing current password changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": "not-the-password", "password": nextPhrase,
		}).AssertValidationError("current_password").AssertJSONPath("code", "invalid_password")
		owner.PatchJSON("/api/v1/user/password", testkit.JSON{"password": nextPhrase}).
			AssertValidationError("current_password").AssertJSONPath("code", "current_password_required")

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": factories.Password,
		}).AssertCreated()
	})

	t.Run("a weak password or one that is the person's email is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": "short",
		}).AssertValidationError("password")
		owner.PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": tc.Owner.Email,
		}).AssertValidationError("password").AssertJSONPath("code", "password_too_common")
	})

	t.Run("a person without a password sets one after confirming", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.WithoutPassword)
		browser := signInByMail(t, tc, user)

		tc.Clock.Travel(confirmationWindow * 2)
		browser.PatchJSON("/api/v1/user/password", testkit.JSON{"password": nextPhrase}).
			AssertStatus(423).AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("a person who signed in just now sets a password without another proof", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.WithoutPassword)
		browser := signInByMail(t, tc, user)

		browser.PatchJSON("/api/v1/user/password", testkit.JSON{"password": nextPhrase}).AssertOK()
		browser.GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("a token cannot change a password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := teammate(t, tc, enums.RoleMember)

		tc.ActingAsToken(member).PatchJSON("/api/v1/user/password", testkit.JSON{
			"current_password": factories.Password, "password": nextPhrase,
		}).AssertForbidden()
	})
}
