package identity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/saml/samltest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

const (
	appURL          = "https://app.example.com"
	connectionsPath = "/api/v1/sso-connections"
	domainsPath     = "/api/v1/organization-domains"
)

/*
company is a workspace whose owner connected a local SAML identity
provider and routed acme.example to it.
*/
type company struct {
	tc         *tests.TestCase
	zone       *dns.Fake
	idp        *samltest.IDP
	connection string
	id         string
}

/*
newCompany starts the application on HTTPS and connects a local
identity provider, active, with newcomers admitted unless jit is false.
*/
func newCompany(t *testing.T, jit bool) company {
	t.Helper()

	tc := tests.New(t, tests.WithEnv("DIMMI_APP__URL="+appURL))
	zone := tc.App.DNS.Fake()
	idp, err := samltest.New("https://idp.example/entity", "https://idp.example/sso", tc.Clock.Now())
	require.NoError(t, err)

	owner := in(tc)
	id := owner.PostJSON(connectionsPath, testkit.JSON{
		"protocol": "saml", "name": "Okta",
		"config": testkit.JSON{
			"idp_entity_id": idp.EntityID, "idp_sso_url": idp.SSOURL,
			"idp_certificates": []string{idp.CertificatePEM()},
			"attribute_map":    testkit.JSON{"email": "email", "display_name": "name", "groups": "groups"},
		},
	}).AssertCreated().String("data.id")
	owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"jit_enabled": jit}).AssertOK()

	added := owner.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated()
	zone.Set(added.String("data.record_name"), dns.Records{TXT: []string{added.String("data.record_value")}})
	domain := added.String("data.id")
	owner.PostJSON(domainsPath+"/"+domain+"/checks", testkit.JSON{}).AssertCreated()
	owner.PatchJSON(domainsPath+"/"+domain, testkit.JSON{"sso_connection_id": id}).AssertOK()

	c := company{tc: tc, zone: zone, idp: idp, id: id}
	c.connection = id
	c.test(t)
	owner.PatchJSON(connectionsPath+"/"+id, testkit.JSON{"status": "active"}).AssertOK()

	return c
}

/*
in returns the owner's browser, working in the workspace.
*/
func in(tc *tests.TestCase) *testkit.Client {
	return tc.ActingAs(tc.Owner).WithHeader("Dimmi-Workspace",
		models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))
}

/*
person is who the identity provider says signs in.
*/
type person struct {
	subject string
	email   string
	name    string
	groups  []string
}

/*
attempt is a sign-in sent to the identity provider: the request it must
answer, the relay state and the browser's attempt cookie.
*/
type attempt struct {
	requestID string
	relay     string
	cookie    *fhttp.Cookie
}

/*
start sends a fresh browser to the identity provider through the
connection.
*/
func (c company) start(t *testing.T) attempt {
	t.Helper()

	res := c.tc.Client().Send(fhttp.MethodGet, "/auth/sso/"+c.id+"/redirect", nil).AssertStatus(fhttp.StatusFound)
	location := res.Header.Get("Location")
	require.True(t, strings.HasPrefix(location, c.idp.SSOURL), location)

	requestID, relay, err := samltest.Request(location)
	require.NoError(t, err)

	return attempt{requestID: requestID, relay: relay, cookie: res.Cookie(c.tc.App.Cookies.SSOName())}
}

/*
answer builds the identity provider's signed answer to the attempt for
the person, changed by the test.
*/
func (c company) answer(t *testing.T, a attempt, p person, change func(*samltest.Response)) string {
	t.Helper()

	attributes := map[string][]string{"email": {p.email}, "name": {p.name}}
	if p.groups != nil {
		attributes["groups"] = p.groups
	}

	r := samltest.Response{
		ACSURL: appURL + "/auth/saml/" + c.id + "/acs", Audience: appURL + "/auth/saml/" + c.id + "/metadata",
		InResponseTo: a.requestID, NameID: p.subject, Attributes: attributes, IssuedAt: c.tc.Clock.Now(),
		Validity: 5 * time.Minute, SignAssertion: true, SignResponse: true,
	}
	if change != nil {
		change(&r)
	}

	encoded, err := c.idp.Encode(r)
	require.NoError(t, err)

	return encoded
}

/*
post posts the answer back as the browser that holds the attempt cookie.
*/
func (c company) post(a attempt, response string) *testkit.Response {
	client := c.tc.Client()
	if a.cookie != nil {
		client = client.WithCookie(a.cookie)
	}

	return client.PostForm("/auth/saml/"+c.id+"/acs", testkit.FormFile{Field: "unused", Name: "unused"},
		map[string]string{"SAMLResponse": response, "RelayState": a.relay})
}

/*
signIn runs a whole sign-in for the person and returns the answer page.
*/
func (c company) signIn(t *testing.T, p person) *testkit.Response {
	t.Helper()

	a := c.start(t)

	return c.post(a, c.answer(t, a, p, nil))
}

/*
test runs an owner's test sign-in through the connection.
*/
func (c company) test(t *testing.T) {
	t.Helper()

	res := in(c.tc).PostJSON(connectionsPath+"/"+c.id+"/tests", testkit.JSON{}).AssertCreated()
	requestID, relay, err := samltest.Request(res.String("data.url"))
	require.NoError(t, err)

	a := attempt{requestID: requestID, relay: relay, cookie: res.Cookie(c.tc.App.Cookies.SSOName())}
	page := c.post(a, c.answer(t, a, person{subject: "owner-1", email: c.tc.Owner.Email}, nil))
	page.AssertStatus(fhttp.StatusSeeOther)
	require.Contains(t, page.Header.Get("Location"), "tested=1")
}

/*
landing returns where an answer page sends the browser.
*/
func landing(res *testkit.Response) string {
	return res.Header.Get("Location")
}
