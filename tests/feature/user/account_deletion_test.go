package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func destroy(client *testkit.Client, email string) *testkit.Response {
	return client.Send(fhttp.MethodDelete, "/api/v1/user", testkit.JSON{"confirm_email": email})
}

func TestUserDeletionPreview(t *testing.T) {
	t.Parallel()

	t.Run("names the workspaces that block and those that would go", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		teammate(t, tc, enums.RoleMember)
		alone, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		owner.GetJSON("/api/v1/user/deletion-preview").AssertOK().
			AssertJSONPath("data.object", "user_deletion_preview").
			AssertJSONPath("data.blocking.#", 1).
			AssertJSONPath("data.blocking.0.id", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).
			AssertJSONPath("data.will_be_deleted.#", 1).
			AssertJSONPath("data.will_be_deleted.0.id", models.PublicID(models.PrefixWorkspace, alone.Workspace.ID))
	})

	t.Run("a workspace with another owner does not stand in the way", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate(t, tc, enums.RoleOwner)

		tc.ActingAsOwner().GetJSON("/api/v1/user/deletion-preview").AssertOK().
			AssertJSONPath("data.blocking.#", 0).
			AssertJSONPath("data.will_be_deleted.#", 0)
	})
}

func TestUserDestroy(t *testing.T) {
	t.Parallel()

	t.Run("a sole owner with teammates is refused and nothing changes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		teammate(t, tc, enums.RoleMember)
		owner := tc.ActingAsOwner()

		destroy(owner, tc.Owner.Email).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "sole_owner").
			AssertJSONPath("workspaces.0", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))

		owner.GetJSON("/api/v1/me").AssertOK()
		tc.Table("users").AssertHas(testkit.Row{"id": tc.Owner.ID, "email": tc.Owner.Email})
	})

	t.Run("the address must be typed correctly", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		destroy(tc.ActingAsOwner(), "someone@else.example").
			AssertValidationError("confirm_email").AssertJSONPath("code", "confirm_email_mismatch")
	})

	t.Run("needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()
		tc.Clock.Travel(confirmationWindow + time.Second)

		destroy(owner, tc.Owner.Email).AssertStatus(fhttp.StatusLocked)
	})

	t.Run("a token cannot delete an account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		destroy(tc.ActingAsToken(tc.Workspace.Owner), tc.Owner.Email).AssertForbidden()
	})

	t.Run("deleting leaves a nameless row and removes every way in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		user, member := teammate(t, tc, enums.RoleMember)
		token := tc.ActingAsToken(member)
		browser := tc.ActingAs(user)
		other := tc.ActingAs(user)
		url := browser.PutFile("/api/v1/user/avatar", "avatar", "a.png", pngPicture(t, 10, 10)).AssertOK().
			String("data.avatar_url")

		res := destroy(browser, user.Email).AssertNoContent()

		assert.Empty(t, res.Cookie(tc.App.Session.CookieName()).Value, "the browser drops the session")
		other.GetJSON("/api/v1/me").AssertUnauthorized()
		token.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": user.Email, "password": factories.Password,
		}).AssertValidationError("email")

		tc.Table("users").AssertHas(testkit.Row{
			"id": user.ID, "name": "Former teammate", "email": "deleted+" + user.ID.String() + "@users.invalid",
			"password_hash": nil, "avatar_path": nil,
		})
		tc.Table("security_events").AssertCount(0, testkit.Row{"user_id": user.ID})
		tc.Table("one_time_tokens").AssertCount(0, testkit.Row{"user_id": user.ID})

		_, told := tests.LastSent[appmail.AccountDeletedMail](tc, fake)
		assert.Equal(t, []string{user.Email}, tests.SentTo(told))
		tc.Drain()
		tc.Client().GetJSON(url).AssertNotFound()
	})

	t.Run("factors are gone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := teammate(t, tc, enums.RoleMember)
		browser := tc.ActingAs(user)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, user)
		require.NoError(t, err)
		tc.Table("user_auth_factors").AssertCount(2, testkit.Row{"user_id": user.ID})

		destroy(browser, user.Email).AssertNoContent()

		tc.Table("user_auth_factors").AssertCount(0, testkit.Row{"user_id": user.ID})
		tc.Table("users").AssertHas(testkit.Row{"id": user.ID, "two_factor_confirmed_at": nil})
	})

	t.Run("re-signup with the same email starts a new account", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := teammate(t, tc, enums.RoleMember)

		destroy(tc.ActingAs(user), user.Email).AssertNoContent()

		fresh := tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Someone New", "email": user.Email, "password": nextPhrase,
		}).AssertCreated()
		assert.NotEqual(t, models.PublicID(models.PrefixUser, user.ID), fresh.String("data.user.id"))

		tc.ActingAsOwner().GetJSON("/api/v1/members/"+models.PublicID(models.PrefixMember, member.ID)).AssertOK().
			AssertJSONPath("data.status", "deactivated").
			AssertJSONPath("data.user.name", "Former teammate")
	})

	t.Run("a workspace the person is alone in goes to the trash and nobody is mailed about it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		destroy(tc.ActingAsOwner(), tc.Owner.Email).AssertNoContent()

		tc.Table("workspaces").AssertMissing(testkit.Row{"id": tc.Workspace.Workspace.ID, "deleted_at": nil})
		sent, _ := tests.Sent[appmail.WorkspaceDeletionScheduledMail](tc, fake)
		assert.Empty(t, sent)
	})

	t.Run("a co-owner keeps the workspace and an invitation the person sent stays valid", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		coOwner, _ := teammate(t, tc, enums.RoleOwner)
		tc.ActingAsOwner().PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{{
			"email": "invitee@example.com", "role": "member",
		}}}).AssertOK()
		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		invite := tc.Fragment(sent.AcceptURL).Get("token")

		destroy(tc.ActingAsOwner(), tc.Owner.Email).AssertNoContent()

		tc.ActingAs(coOwner).
			WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)).
			GetJSON("/api/v1/workspace").AssertOK()
		tc.Client().WithHeader("Dimmi-Invitation-Token", invite).GetJSON("/api/v1/invitations/preview").AssertOK()
		invitee := tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Invitee", "email": "invitee@example.com", "password": nextPhrase, "invitation_token": invite,
		}).AssertCreated()
		tc.Browser(invitee).GetJSON("/api/v1/user/memberships").AssertOK().AssertJSONPath("data.#", 1)
	})
}
