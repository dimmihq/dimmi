package identity_test

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

/*
enforced connects the company and requires its identity provider, the
owner proving a test sign-in from the last day.
*/
func enforced(t *testing.T) company {
	t.Helper()

	c := newCompany(t, true)
	in(c.tc).PatchJSON(connectionsPath+"/"+c.id, testkit.JSON{
		"enforced": true, "confirm_owners_can_sign_in": true,
	}).AssertOK()

	return c
}

/*
teammate adds a person at acme.example to the workspace with the role.
*/
func teammate(t *testing.T, c company, email string, role enums.Role) {
	t.Helper()

	user := c.tc.CreateUser(func(u models.User) models.User { u.Email = email; return u })
	_, err := factories.Member(t.Context(), c.tc.App, factories.Membership{
		Workspace: c.tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)
}

/*
password signs in with the password at the address.
*/
func password(c company, email string, bypass bool) *testkit.Response {
	return c.tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": email, "password": factories.Password, "sso_bypass": bypass,
	})
}

func TestSSOEnforcement(t *testing.T) {
	t.Parallel()

	t.Run("enforcement sends password sign-in to the idp", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)
		teammate(t, c, "ada@acme.example", enums.RoleMember)

		password(c, "ada@acme.example", false).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "sso_required").
			AssertJSONPath("redirect_url", "/auth/sso/"+c.id+"/redirect")
	})

	t.Run("an unknown address under the domain gets the same answer", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)

		password(c, "nobody@acme.example", false).AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "sso_required")
	})

	t.Run("magic link and reset are refused under enforcement", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)
		teammate(t, c, "ada@acme.example", enums.RoleMember)

		c.tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": "ada@acme.example"}).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "sso_required")
		fake := c.tc.Mail()
		c.tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": "ada@acme.example"}).
			AssertStatus(fhttp.StatusAccepted)
		c.tc.Drain()
		sent, _ := tests.Sent[appmail.ResetPasswordMail](c.tc, fake)
		assert.Empty(t, sent)
	})

	t.Run("the sign-in page learns where an address signs in", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)

		c.tc.Client().PostJSON("/api/v1/sso-lookups", testkit.JSON{"email": "anyone@acme.example"}).AssertOK().
			AssertJSONPath("data.redirect_url", "/auth/sso/"+c.id+"/redirect").
			AssertJSONPath("data.name", "Okta").AssertJSONPath("data.enforced", true)
		c.tc.Client().PostJSON("/api/v1/sso-lookups", testkit.JSON{"email": "anyone@elsewhere.example"}).AssertOK().
			AssertJSONPath("data.redirect_url", nil)
	})

	t.Run("an owner can bypass, every owner hears of it, and it lasts one hour", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)
		fake := c.tc.Mail()
		teammate(t, c, "boss@acme.example", enums.RoleOwner)

		res := password(c, "boss@acme.example", true)
		res.AssertStatus(fhttp.StatusCreated)
		session := res.Cookie(c.tc.App.Session.CookieName())
		workspace := models.PublicID(models.PrefixWorkspace, c.tc.Workspace.Workspace.ID)
		browser := c.tc.Client().WithCookie(session).WithHeader("Dimmi-Workspace", workspace)
		browser.GetJSON("/api/v1/members").AssertOK()

		c.tc.Drain()
		sent, _ := tests.Sent[appmail.SSOBypassUsedMail](c.tc, fake)
		assert.Len(t, sent, 2)

		c.tc.Clock.Advance(61 * time.Minute)
		browser.GetJSON("/api/v1/members").AssertStatus(fhttp.StatusForbidden)
	})

	t.Run("a non owner bypass looks like a wrong password", func(t *testing.T) {
		t.Parallel()
		c := enforced(t)
		teammate(t, c, "ada@acme.example", enums.RoleMember)

		refused := password(c, "ada@acme.example", true)
		wrong := c.tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": "nobody@elsewhere.example", "password": "wrong",
		})

		assert.Equal(t, wrong.Status, refused.Status)
		refused.AssertJSONPath("code", wrong.String("code"))
	})
}
