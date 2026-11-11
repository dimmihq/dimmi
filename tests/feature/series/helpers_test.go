package series_test

import (
	"encoding/json"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
inboundSecret is the secret of the test inbound source.
*/
const inboundSecret = "series-inbound-secret"

/*
desk is a workspace ready to run series that send email: the test
case, its owner, the connected address on a verified domain, and the
owner's member id.
*/
type desk struct {
	tc         *tests.TestCase
	owner      *testkit.Client
	addressID  string
	forwarding string
	member     string
}

/*
newDesk boots the application sending through Mailpit, with
support@acme.test connected and its domain verified.
*/
func newDesk(t *testing.T, env ...string) desk {
	t.Helper()

	server, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)

	vars := []string{
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp",
		"DIMMI_MAIL__MAILERS__CUSTOMER__HOST=" + server.Hostname(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__PORT=" + server.Port(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
		"DIMMI_CHANNELS__EMAIL__SENDING__RELAY_SPF_INCLUDE=include:spf.relay.test",
		"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=in.series.test",
		"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test",
		"DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET=" + inboundSecret,
		"DIMMI_SERIES__MAX_MESSAGES_PER_DAY=100",
	}
	vars = append(vars, env...)

	tc := tests.New(t, tests.WithEnv(vars...))
	owner := tc.ActingAsOwner()
	address := owner.PostJSON("/api/v1/email-addresses", testkit.JSON{
		"address": "support@acme.test", "display_name": "Acme",
	}).AssertCreated()

	d := desk{
		tc: tc, owner: owner, addressID: address.String("data.id"),
		forwarding: address.String("data.forwarding_address"),
		member:     models.PublicID(models.PrefixMember, tc.Workspace.Owner.ID),
	}
	d.verifyDomain(tc.App.DNS.Fake())

	return d
}

/*
verifyDomain adds acme.test, publishes its records and checks it.
*/
func (d desk) verifyDomain(zone *dns.Fake) {
	d.tc.T.Helper()

	created := d.owner.PostJSON("/api/v1/email-domains", testkit.JSON{"domain": "acme.test"}).AssertCreated()

	var body struct {
		Data struct {
			ID      string `json:"id"`
			Records []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"records"`
		} `json:"data"`
	}
	created.Decode(&body)

	txt := map[string][]string{}
	mx := map[string][]dns.MX{}

	for _, r := range body.Data.Records {
		switch {
		case r.Value == "":
		case r.Type == "CNAME":
			zone.Set(r.Name, dns.Records{CNAME: r.Value}).
				Set(r.Value, dns.Records{Addrs: []netip.Addr{netip.MustParseAddr("192.0.2.10")}})
		case r.Type == "MX":
			mx[r.Name] = append(mx[r.Name], dns.MX{Host: r.Value, Pref: 10})
		default:
			txt[r.Name] = append(txt[r.Name], r.Value)
		}
	}

	for name := range txt {
		zone.Set(name, dns.Records{TXT: txt[name], MX: mx[name]})
	}

	for name := range mx {
		zone.Set(name, dns.Records{TXT: txt[name], MX: mx[name]})
	}

	d.owner.PostJSON("/api/v1/email-domains/"+body.Data.ID+"/checks", nil).AssertStatus(202)
	d.tc.Drain()
	d.owner.GetJSON("/api/v1/email-domains/"+body.Data.ID).AssertJSONPath("data.status", "verified")
}

/*
contact adds a customer with custom attributes and returns their id.
*/
func (d desk) contact(email, name string, attrs testkit.JSON) string {
	d.tc.T.Helper()

	body := testkit.JSON{"name": name, "custom_attributes": attrs}
	if email != "" {
		body["email"] = email
	}

	return d.owner.PostJSON("/api/v1/contacts", body).AssertCreated().String("data.id")
}

/*
series makes a series from a definition, saves the sending settings and
returns its id.
*/
func (d desk) series(name, subject string, spec testkit.JSON) string {
	d.tc.T.Helper()

	created := d.owner.PostJSON("/api/v1/series", testkit.JSON{"name": name, "subject": subject, "spec": spec}).
		AssertCreated()
	id := created.String("data.id")

	d.owner.PatchJSON("/api/v1/series/"+id, testkit.JSON{
		"lock_version": num(created, "data.lock_version"), "name": name,
		"email_address_id": d.addressID, "sender_id": d.member, "reply_member_id": d.member,
	}).AssertOK()

	return id
}

/*
publish sets the series live and runs what it queues.
*/
func (d desk) publish(id string) {
	d.tc.T.Helper()

	d.owner.PostJSON("/api/v1/series/"+id+"/publications", testkit.JSON{"note": "go"}).AssertOK()
	d.drain()
}

/*
drain runs queued jobs, and those waiting for later, a few rounds.
*/
func (d desk) drain() {
	for range 4 {
		d.tc.DrainLater()
	}
}

/*
journeys lists the series' journeys, newest first.
*/
func (d desk) journeys(id string) *testkit.Response {
	d.tc.T.Helper()

	return d.owner.GetJSON("/api/v1/series/" + id + "/enrollments").AssertOK()
}

/*
report reads the series' report.
*/
func (d desk) report(id string) *testkit.Response {
	d.tc.T.Helper()

	return d.owner.GetJSON("/api/v1/series/" + id + "/report").AssertOK()
}

/*
proRules matches contacts on the pro plan.
*/
func proRules() testkit.JSON {
	return testkit.JSON{"rules": testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"}}
}

/*
email is an email step.
*/
func email(id, subject string) testkit.JSON {
	return testkit.JSON{"id": id, "kind": "email", "settings": testkit.JSON{
		"subject": subject, "body": "Hi {{ contact.first_name | default: \"there\" }},\n\nWelcome aboard.",
	}}
}

/*
edge leads from one step to another.
*/
func edge(from, path, to string) testkit.JSON {
	return testkit.JSON{"from": from, "path": path, "to": to}
}

/*
mailCount returns how many emails Mailpit holds for the recipient.
*/
func mailCount(t *testing.T, recipient string) int {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)
	api := testkit.Env(t, "MAILPIT_API_URL")

	resp, err := client.New().Get(t.Context(), api+"/api/v1/search?query="+url.QueryEscape(`to:"`+recipient+`"`))
	require.NoError(t, err)

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))

	return len(found.Messages)
}

/*
unique returns an address no other test uses.
*/
func unique(name string) string {
	return name + "." + strings.ToLower(testkit.UniqueName("")) + "@customer.test"
}

/*
num reads a number from a response at a dotted path, zero when it is
missing.
*/
func num(r *testkit.Response, path string) int64 {
	var node any
	if json.Unmarshal(r.Body, &node) != nil {
		return 0
	}

	for key := range strings.SplitSeq(path, ".") {
		switch v := node.(type) {
		case map[string]any:
			node = v[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i >= len(v) {
				return 0
			}

			node = v[i]
		default:
			return 0
		}
	}

	n, _ := node.(float64)

	return int64(n)
}

/*
reply answers the newest series email the recipient got, through the
test inbound source, and runs what it queues.
*/
func (d desk) reply(recipient, text string) {
	d.tc.T.Helper()

	var token, messageID string

	ctx := d.tc.T.Context()
	err := d.tc.Operator().QueryRow(ctx, `SELECT reply_token, coalesce(message_id_header, '')
		FROM broadcast_recipients WHERE lower(address) = lower($1) ORDER BY created_at DESC LIMIT 1`, recipient).
		Scan(&token, &messageID)
	require.NoError(d.tc.T, err)

	raw := "From: <" + recipient + ">\r\nTo: Acme <" + d.forwarding + ">\r\nSubject: Re: Welcome\r\n" +
		"Message-ID: <reply." + testkit.UniqueName("m") + "@customer.test>\r\nIn-Reply-To: <" + messageID + ">\r\n" +
		"\r\n" + text + "\r\n"
	to := strings.Replace(d.forwarding, "@", "+"+token+"@", 1)

	d.tc.Client().WithHeader("Authorization", "Bearer "+inboundSecret).
		Send("POST", "/webhooks/test/inbound?to="+url.QueryEscape(to), []byte(raw)).AssertStatus(202)
	d.drain()
}
