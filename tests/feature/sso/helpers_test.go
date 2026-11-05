package sso_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const (
	connectionsPath = "/api/v1/sso-connections"
	domainsPath     = "/api/v1/organization-domains"
)

/*
secure starts the application on an HTTPS address, which single sign-on
needs, with DNS answered by a fake zone.
*/
func secure(t *testing.T) (*tests.TestCase, *dns.Fake) {
	t.Helper()

	tc := tests.New(t, tests.WithEnv("DIMMI_APP__URL=https://app.example.com"))

	return tc, tc.App.DNS.Fake()
}

/*
in returns a browser of the user working in the test case's workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)

	return tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspace)
}

/*
join adds a person to the test case's workspace with the role.
*/
func join(t *testing.T, tc *tests.TestCase, role enums.Role) models.User {
	t.Helper()

	user := tc.CreateUser()
	_, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user
}

/*
samlConnection creates a draft SAML connection as the owner and returns
its public id.
*/
func samlConnection(tc *tests.TestCase) string {
	tc.T.Helper()

	return in(tc, tc.Owner).PostJSON(connectionsPath, testkit.JSON{
		"protocol": "saml", "name": "Okta",
		"config": testkit.JSON{"idp_entity_id": "http://idp.example/okta", "idp_sso_url": "https://idp.example/sso"},
	}).AssertCreated().String("data.id")
}

/*
verifiedDomain adds acme.example as the owner, publishes its proof in the
fake zone, checks it and returns its public id.
*/
func verifiedDomain(tc *tests.TestCase, zone *dns.Fake) string {
	tc.T.Helper()

	owner := in(tc, tc.Owner)
	added := owner.PostJSON(domainsPath, testkit.JSON{"domain": "acme.example"}).AssertCreated()
	zone.Set(added.String("data.record_name"), dns.Records{TXT: []string{added.String("data.record_value")}})
	id := added.String("data.id")
	owner.PostJSON(domainsPath+"/"+id+"/checks", testkit.JSON{}).AssertCreated().
		AssertJSONPath("data.status", "verified")

	return id
}

/*
markTested records a successful test sign-in on the connection, as the
identity provider round trip does.
*/
func markTested(t *testing.T, tc *tests.TestCase, connection string) {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixSSOConnection), connection)
	require.NoError(t, err)
	_, err = tc.Operator().Exec(t.Context(),
		`UPDATE sso_connections SET status = 'tested', tested_at = $2 WHERE id = $1`, id, tc.Clock.Now())
	require.NoError(t, err)
}

/*
signedInThrough marks every session of the user as signed in through the
connection, as a sign-in at the identity provider would.
*/
func signedInThrough(t *testing.T, tc *tests.TestCase, user models.User, connection string) {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixSSOConnection), connection)
	require.NoError(t, err)
	_, err = tc.Operator().Exec(t.Context(),
		`UPDATE sessions SET sso_connection_id = $2 WHERE user_id = $1`, user.ID, id)
	require.NoError(t, err)
}
