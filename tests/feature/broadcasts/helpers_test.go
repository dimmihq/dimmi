package broadcasts_test

import (
	"context"
	"maps"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
The test inbound domain and the secret of the test inbound source.
*/
const (
	inboundDomain = "in.broadcasts.test"
	inboundSecret = "broadcasts-inbound-secret"
)

/*
desk is a workspace ready to send email broadcasts: the test case, its
owner and the connected address on a verified domain.
*/
type desk struct {
	tc         *tests.TestCase
	owner      *testkit.Client
	addressID  string
	forwarding string
}

/*
newDesk boots the application sending through Mailpit and receiving
through the test source, with support@acme.test connected and its
domain verified.
*/
func newDesk(t *testing.T, env ...string) desk {
	t.Helper()

	server, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)

	vars := append([]string{
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp",
		"DIMMI_MAIL__MAILERS__CUSTOMER__HOST=" + server.Hostname(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__PORT=" + server.Port(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
		"DIMMI_CHANNELS__EMAIL__SENDING__RELAY_SPF_INCLUDE=include:spf.relay.test",
		"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=" + inboundDomain,
		"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test",
		"DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET=" + inboundSecret,
	}, env...)

	tc := tests.New(t, tests.WithEnv(vars...))
	owner := tc.ActingAsOwner()
	address := owner.PostJSON("/api/v1/email-addresses", testkit.JSON{
		"address": "support@acme.test", "display_name": "Acme",
	}).AssertCreated()

	d := desk{
		tc: tc, owner: owner, addressID: address.String("data.id"),
		forwarding: address.String("data.forwarding_address"),
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
contact adds a customer on a plan.
*/
func (d desk) contact(email, name, plan string) {
	d.tc.T.Helper()

	body := testkit.JSON{"name": name, "custom_attributes": testkit.JSON{"plan": plan}}
	if email != "" {
		body["email"] = email
	}

	d.owner.PostJSON("/api/v1/contacts", body).AssertCreated()
}

/*
emailBroadcast writes an email broadcast to everyone on the pro plan and
returns its id.
*/
func (d desk) emailBroadcast(name string, extra testkit.JSON) string {
	d.tc.T.Helper()

	id := d.owner.PostJSON("/api/v1/broadcasts", testkit.JSON{"name": name, "channel": "email"}).
		AssertCreated().String("data.id")

	proPlan := testkit.JSON{"field": "contact.custom.plan", "op": "eq", "value": "pro"}
	body := testkit.JSON{
		"subject":          "What's new for {{ contact.first_name | default: \"you\" }}",
		"preview":          "Three things we shipped.",
		"body":             "Hi {{ contact.first_name | default: \"there\" }},\n\nRead [the changelog](https://acme.test/changelog).",
		"rules":            testkit.JSON{"rules": proPlan},
		"email_address_id": d.addressID,
		"sender_id":        models.PublicID(models.PrefixMember, d.tc.Workspace.Owner.ID),
	}
	maps.Copy(body, extra)

	d.owner.PatchJSON("/api/v1/broadcasts/"+id, body).AssertOK()

	return id
}

/*
send sends the broadcast and runs every job it queues until it is done.
*/
func (d desk) send(id string) {
	d.tc.T.Helper()

	d.owner.PostJSON("/api/v1/broadcasts/"+id+"/send", nil).AssertOK()
	d.drain()
}

/*
drain runs queued jobs, and those waiting for later, until the
dispatcher has had its rounds.
*/
func (d desk) drain() {
	for range 6 {
		d.tc.DrainLater()
	}
}

/*
report reads the broadcast's report.
*/
func (d desk) report(id string) *testkit.Response {
	d.tc.T.Helper()

	return d.owner.GetJSON("/api/v1/broadcasts/" + id + "/report").AssertOK()
}

/*
deliver hands a raw email to the test inbound source and runs what it
queues.
*/
func (d desk) deliver(raw, to string) {
	d.tc.T.Helper()

	d.tc.Client().WithHeader("Authorization", "Bearer "+inboundSecret).
		Send("POST", "/webhooks/test/inbound?to="+url.QueryEscape(to), []byte(raw)).AssertStatus(202)
	d.drain()
}

/*
recipients returns the broadcast's recipient rows.
*/
func (d desk) recipients(id string) []models.BroadcastRecipient {
	d.tc.T.Helper()

	bid, err := d.tc.App.IDs.Decode(string(models.PrefixBroadcast), id)
	require.NoError(d.tc.T, err)

	var out []models.BroadcastRecipient

	ws := d.tc.Workspace.Workspace.ID
	ctx := tenancy.With(d.tc.T.Context(), ws.String())
	err = d.tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := tx.Query(ctx, "SELECT id FROM broadcast_recipients WHERE broadcast_id = $1 ORDER BY id", bid)
		if err != nil {
			return err
		}

		var list []ids.UUID
		for rows.Next() {
			var id ids.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}

			list = append(list, id)
		}

		rows.Close()

		for _, rid := range list {
			r, err := models.New(tx).FindBroadcastRecipient(ctx, models.FindBroadcastRecipientParams{
				WorkspaceID: ws, ID: rid,
			})
			if err != nil {
				return err
			}

			out = append(out, r)
		}

		return nil
	})
	require.NoError(d.tc.T, err)

	return out
}

/*
mailFor returns the raw headers and text of the one email Mailpit holds
for the recipient.
*/
func mailFor(t *testing.T, recipient string) (string, string) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)
	api := testkit.Env(t, "MAILPIT_API_URL")

	resp, err := client.New().Get(t.Context(), api+"/api/v1/search?query="+url.QueryEscape(`to:"`+recipient+`"`))
	require.NoError(t, err)

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))
	require.Len(t, found.Messages, 1)

	headers, err := client.New().Get(t.Context(), api+"/api/v1/message/"+found.Messages[0].ID+"/headers")
	require.NoError(t, err)

	var msg struct{ Text string }
	body, err := client.New().Get(t.Context(), api+"/api/v1/message/"+found.Messages[0].ID)
	require.NoError(t, err)
	require.NoError(t, body.JSON(&msg))

	return string(headers.Body), msg.Text
}

/*
unique returns an address no other test uses.
*/
func unique(name string) string {
	return name + "." + strings.ToLower(testkit.UniqueName("")) + "@customer.test"
}
