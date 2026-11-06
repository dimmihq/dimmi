package identity_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
issuerCompany is a workspace whose owner connected a local OpenID
Connect issuer, tested and active, with acme.example routed to it.
*/
type issuerCompany struct {
	tc  *tests.TestCase
	srv *drivertest.Server
	id  string
}

/*
newIssuerCompany connects an issuer that signs in the person.
*/
func newIssuerCompany(t *testing.T, person socialite.User) issuerCompany {
	t.Helper()

	srv := drivertest.NewServer(t, person)
	tc := tests.New(t, tests.WithEnv("DIMMI_APP__URL="+appURL, "DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true"))
	zone := tc.App.DNS.Fake()
	owner := in(tc)

	id := owner.PostJSON(connectionsPath, testkit.JSON{
		"protocol": "oidc", "name": "Entra", "client_secret": srv.ClientSecret,
		"config": testkit.JSON{"issuer": srv.URL, "client_id": srv.ClientID},
	}).AssertCreated().String("data.id")
	owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"jit_enabled": true}).AssertOK()

	added := owner.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated()
	zone.Set(added.String("data.record_name"), dns.Records{TXT: []string{added.String("data.record_value")}})
	domain := added.String("data.id")
	owner.PostJSON(domainsPath+"/"+domain+"/checks", testkit.JSON{}).AssertCreated()
	owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()

	c := issuerCompany{tc: tc, srv: srv, id: id}
	test := owner.PostJSON(connectionsPath+"/"+id+"/tests", testkit.JSON{}).AssertCreated()
	page := c.back(t, test.String("data.url"), test.Cookie(tc.App.Cookies.SSOName()))
	require.Contains(t, landing(page), "tested=1")
	owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()

	return c
}

/*
back approves the sign-in at the issuer and follows its callback with
the browser's attempt cookie.
*/
func (c issuerCompany) back(t *testing.T, authURL string, cookie *fhttp.Cookie) *testkit.Response {
	t.Helper()

	callback, err := url.Parse(c.srv.Approve(t, authURL))
	require.NoError(t, err)

	client := c.tc.Client()
	if cookie != nil {
		client = client.WithCookie(cookie)
	}

	return client.Send(fhttp.MethodGet, callback.RequestURI(), nil)
}

/*
start sends a fresh browser to the issuer.
*/
func (c issuerCompany) start(t *testing.T) (string, *fhttp.Cookie) {
	t.Helper()

	res := c.tc.Client().Send(fhttp.MethodGet, "/auth/sso/"+c.id+"/redirect", nil).AssertStatus(fhttp.StatusFound)

	return res.Header.Get("Location"), res.Cookie(c.tc.App.Cookies.SSOName())
}

func TestOIDCSignIn(t *testing.T) {
	t.Parallel()

	ada := socialite.User{ID: "ada-sub", Email: "ada@acme.example", EmailVerified: true, Name: "Ada Lovelace"}

	t.Run("sign in through an openid issuer starts a session", func(t *testing.T) {
		t.Parallel()
		c := newIssuerCompany(t, ada)
		authURL, cookie := c.start(t)

		res := c.back(t, authURL, cookie)

		assert.Equal(t, "/d", landing(res))
		session := res.Cookie(c.tc.App.Session.CookieName())
		c.tc.Client().WithCookie(session).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.email", "ada@acme.example")
	})

	t.Run("a callback without the browser's attempt cookie never signs in", func(t *testing.T) {
		t.Parallel()
		c := newIssuerCompany(t, ada)
		authURL, _ := c.start(t)

		assert.Equal(t, "/login?error=sso_failed", landing(c.back(t, authURL, nil)))
	})

	t.Run("an address outside the verified domain is refused", func(t *testing.T) {
		t.Parallel()
		c := newIssuerCompany(t, socialite.User{ID: "eve", Email: "eve@elsewhere.example", EmailVerified: true})
		authURL, cookie := c.start(t)

		assert.Equal(t, "/login?error=sso_email_domain_not_verified", landing(c.back(t, authURL, cookie)))
	})

	t.Run("the client secret is never shown", func(t *testing.T) {
		t.Parallel()
		c := newIssuerCompany(t, ada)

		got := in(c.tc).GetJSON(connectionsPath + "/" + c.id).AssertOK()
		got.AssertJSONPath("data.has_secret", true)
		assert.NotContains(t, string(got.Body), c.srv.ClientSecret)
	})
}
