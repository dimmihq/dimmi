package identity_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/saml/samltest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestSAMLSignIn(t *testing.T) {
	t.Parallel()

	t.Run("sign in through saml starts a session", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)

		res := c.signIn(t, person{subject: "ada-1", email: "ada@acme.example", name: "Ada Lovelace"})

		res.AssertStatus(fhttp.StatusSeeOther)
		assert.Equal(t, "/d", landing(res))
		session := res.Cookie(c.tc.App.Session.CookieName())
		require.NotNil(t, session)
		me := c.tc.Client().WithCookie(session).GetJSON("/api/v1/me").AssertOK()
		me.AssertJSONPath("data.user.email", "ada@acme.example")
		me.AssertJSONPath("data.user.name", "Ada Lovelace")
	})

	t.Run("jit creates the member with the default role", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)

		c.signIn(t, person{subject: "ada-1", email: "ada@acme.example", name: "Ada"}).AssertStatus(fhttp.StatusSeeOther)

		in(c.tc).GetJSON("/api/v1/members?q=ada@acme.example").AssertOK().
			AssertJSONPath("data.0.role", "member").AssertJSONPath("data.0.user.email", "ada@acme.example")
	})

	t.Run("without jit an unknown person gets no access", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, false)

		res := c.signIn(t, person{subject: "ada-1", email: "ada@acme.example"})

		assert.Equal(t, "/login?error=sso_no_access", landing(res))
	})

	t.Run("an address outside the verified domain is refused", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)

		res := c.signIn(t, person{subject: "eve-1", email: "eve@elsewhere.example"})

		assert.Equal(t, "/login?error=sso_email_domain_not_verified", landing(res))
	})

	t.Run("a replayed assertion is refused", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		a := c.start(t)
		answer := c.answer(t, a, person{subject: "ada-1", email: "ada@acme.example"}, nil)
		c.post(a, answer).AssertStatus(fhttp.StatusSeeOther)

		again := c.start(t)
		res := c.post(attempt{relay: again.relay, cookie: again.cookie}, answer)

		assert.Equal(t, "/login?error=sso_failed", landing(res))
	})

	t.Run("an answer without the browser's attempt cookie never signs in", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		a := c.start(t)
		answer := c.answer(t, a, person{subject: "ada-1", email: "ada@acme.example"}, nil)

		res := c.post(attempt{relay: a.relay}, answer)

		assert.Equal(t, "/login?error=sso_failed", landing(res))
		assert.NotContains(t, strings.Join(res.Header.Values("Set-Cookie"), ";"), c.tc.App.Session.CookieName()+"=")
	})

	t.Run("idp initiated sign in restarts from dimmi", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		answer := c.answer(t, attempt{}, person{subject: "ada-1", email: "ada@acme.example"}, nil)

		res := c.post(attempt{}, answer)

		assert.Equal(t, "/auth/sso/"+c.id+"/redirect", landing(res))
	})

	t.Run("a tampered answer is refused and the owners see why", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		a := c.start(t)
		answer := c.answer(t, a, person{subject: "ada-1", email: "ada@acme.example"}, func(r *samltest.Response) {
			r.WrapSignature = true
		})

		assert.Equal(t, "/login?error=sso_failed", landing(c.post(a, answer)))
		in(c.tc).GetJSON(connectionsPath+"/"+c.id).AssertOK().
			AssertJSONPath("data.last_error.code", "invalid_signature")
	})

	t.Run("a deactivated member stays out", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		user := c.tc.CreateUser(func(u models.User) models.User { u.Email = "ada@acme.example"; return u })
		member, err := factories.Member(t.Context(), c.tc.App, factories.Membership{
			Workspace: c.tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		_, err = c.tc.Operator().Exec(t.Context(),
			`UPDATE workspace_members SET status = 'deactivated', deactivated_at = now() WHERE id = $1`, member.ID)
		require.NoError(t, err)

		res := c.signIn(t, person{subject: "ada-1", email: "ada@acme.example"})

		assert.Equal(t, "/login?error=membership_deactivated", landing(res))
	})

	t.Run("group mapping sets the role and never touches an owner", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		in(c.tc).PatchJSON(connectionsPath+"/"+c.id, testkit.JSON{
			"group_role_map": []testkit.JSON{{"group": "admins", "role": "admin"}},
		}).AssertOK()

		c.signIn(t, person{subject: "ada-1", email: "ada@acme.example", groups: []string{"admins"}}).
			AssertStatus(fhttp.StatusSeeOther)
		c.signIn(t, person{subject: "owner-1", email: c.tc.Owner.Email, groups: []string{"admins"}})

		in(c.tc).GetJSON("/api/v1/members?q=ada@acme.example").AssertOK().AssertJSONPath("data.0.role", "admin")
		in(c.tc).GetJSON("/api/v1/members/me").AssertOK().AssertJSONPath("data.role", "owner")
	})

	t.Run("an idp cannot sign in a person from another company", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		other := newCompany(t, true)
		c.signIn(t, person{subject: "ada-1", email: "ada@acme.example"}).AssertStatus(fhttp.StatusSeeOther)

		a := c.start(t)
		forged, err := other.idp.Encode(samltest.Response{
			ACSURL: appURL + "/auth/saml/" + c.id + "/acs", Audience: appURL + "/auth/saml/" + c.id + "/metadata",
			InResponseTo: a.requestID, NameID: "ada-1", Attributes: map[string][]string{"email": {"ada@acme.example"}},
			IssuedAt: c.tc.Clock.Now(), SignAssertion: true, SignResponse: true,
		})
		require.NoError(t, err)

		assert.Equal(t, "/login?error=sso_failed", landing(c.post(a, forged)))
	})

	t.Run("test sign in records the result and starts no session", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)

		in(c.tc).GetJSON(connectionsPath+"/"+c.id).AssertOK().AssertJSONPath("data.status", "active")
		assert.NotEmpty(t, in(c.tc).GetJSON(connectionsPath+"/"+c.id).String("data.tested_at"))
	})

	t.Run("the metadata describes this side", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)

		res := c.tc.Client().Send(fhttp.MethodGet, "/auth/saml/"+c.id+"/metadata", nil).AssertOK()

		assert.Contains(t, res.Header.Get("Content-Type"), "samlmetadata")
		assert.Contains(t, string(res.Body), appURL+"/auth/saml/"+c.id+"/acs")
	})
}
