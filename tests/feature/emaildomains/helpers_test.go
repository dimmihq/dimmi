package emaildomains_test

import (
	"context"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The inbound domain every test receives on, and the SES API the tests
stand in for.
*/
const (
	inboundDomain = "inbound.test"
	sesBase       = "https://email.eu-west-1.amazonaws.com/v2/email"
)

/*
sesEnv sends the customer stream through SES in eu-west-1.
*/
func sesEnv() tests.Option {
	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=ses",
		"DIMMI_MAIL__MAILERS__CUSTOMER__REGION=eu-west-1",
		"DIMMI_MAIL__MAILERS__CUSTOMER__ACCESS_KEY_ID=AKID",
		"DIMMI_MAIL__MAILERS__CUSTOMER__SECRET_ACCESS_KEY=secret",
	)
}

/*
relayEnv sends the customer stream through an SMTP relay we sign for.
*/
func relayEnv(t *testing.T) tests.Option {
	t.Helper()

	server, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)

	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp",
		"DIMMI_MAIL__MAILERS__CUSTOMER__HOST="+server.Hostname(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__PORT="+server.Port(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
		"DIMMI_CHANNELS__EMAIL__SENDING__RELAY_SPF_INCLUDE=include:spf.relay.test",
	)
}

/*
domainCase boots the application receiving mail on the test inbound
domain, with DNS answered by a fake zone.
*/
func domainCase(t *testing.T, opts ...tests.Option) (*tests.TestCase, *dns.Fake) {
	t.Helper()

	env := tests.WithEnv("DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=" + inboundDomain)
	tc := tests.New(t, append([]tests.Option{env}, opts...)...)

	return tc, tc.App.DNS.Fake()
}

/*
identity is the state SES reports for a domain.
*/
type identity struct {
	dkim     string
	mailFrom string
}

/*
stubSES answers the SES identity calls for acme.test with the identity's
state, and returns the fake so requests can be checked.
*/
func stubSES(tc *tests.TestCase, state identity) *httpclienttest.Fake {
	tc.T.Helper()

	const domain = "acme.test"

	body := `{"VerifiedForSendingStatus":` + map[bool]string{true: "true", false: "false"}[state.dkim == "SUCCESS"] +
		`,"DkimAttributes":{"Tokens":["tok1","tok2","tok3"],"SigningHostedZone":"dkim.amazonses.com",` +
		`"Status":"` + state.dkim + `"},"MailFromAttributes":{"MailFromDomain":"bounce.` + domain +
		`","MailFromDomainStatus":"` + state.mailFrom + `"}}`
	ok := httpclient.Response{Header: fhttp.Header{"Content-Type": {"application/json"}}, Body: []byte("{}")}
	got := ok
	got.Body = []byte(body)

	return httpclienttest.New(tc.App.HTTP).PreventStrayRequests().
		Stub("POST "+sesBase+"/identities", ok).
		Stub("GET "+sesBase+"/identities/"+domain, got).
		Stub("PUT "+sesBase+"/identities/"+domain+"/mail-from", ok).
		Stub("PUT "+sesBase+"/identities/"+domain+"/dkim/signing", ok)
}

/*
record is one record of a domain as the API answers.
*/
type record struct {
	Purpose  string `json:"purpose"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

/*
domainBody is a domain as the API answers.
*/
type domainBody struct {
	Data struct {
		ID       string   `json:"id"`
		Status   string   `json:"status"`
		Records  []record `json:"records"`
		Warnings []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"warnings"`
	} `json:"data"`
}

/*
decode reads a domain from a response.
*/
func decode(res *testkit.Response) domainBody {
	var body domainBody
	res.Decode(&body)

	return body
}

/*
publish puts every record of the domain into the zone.
*/
func publish(zone *dns.Fake, body domainBody) {
	txt := map[string][]string{}
	mx := map[string][]dns.MX{}

	for _, r := range body.Data.Records {
		if r.Value == "" {
			continue
		}

		switch r.Type {
		case "CNAME":
			zone.Set(r.Name, dns.Records{CNAME: r.Value}).
				Set(r.Value, dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}})
		case "MX":
			mx[r.Name] = append(mx[r.Name], dns.MX{Host: r.Value, Pref: 10})
		default:
			txt[r.Name] = append(txt[r.Name], r.Value)
		}
	}

	for name := range merge(txt, mx) {
		zone.Set(name, dns.Records{TXT: txt[name], MX: mx[name]})
	}
}

/*
merge returns every name holding text or mail servers.
*/
func merge(txt map[string][]string, mx map[string][]dns.MX) map[string]bool {
	names := map[string]bool{}
	for name := range txt {
		names[name] = true
	}

	for name := range mx {
		names[name] = true
	}

	return names
}

/*
check runs one check of the domain now, as the queue would.
*/
func check(tc *tests.TestCase, workspace ids.UUID, public string) {
	tc.T.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixEmailDomain), public)
	require.NoError(tc.T, err)

	checker := &email.CheckEmailDomain{Domains: tc.App.EmailDomains}
	require.NoError(tc.T, checker.Handle(tc.T.Context(), workspace, id))
}

/*
query runs the function with the workspace's queries.
*/
func query[T any](
	tc *tests.TestCase, workspace ids.UUID, fn func(ctx context.Context, q *models.Queries) (T, error),
) T {
	tc.T.Helper()

	var out T

	err := tc.App.DB.Transaction(tenancy.With(tc.T.Context(), workspace.String()),
		func(ctx context.Context, tx *database.Tx) error {
			var err error

			out, err = fn(ctx, models.New(tx))

			return err
		})
	require.NoError(tc.T, err)

	return out
}

/*
mustID returns the internal id of a domain's public id.
*/
func mustID(tc *tests.TestCase, public string) ids.UUID {
	tc.T.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixEmailDomain), public)
	require.NoError(tc.T, err)

	return id
}
