package resend_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

func newDomains(client *httpclient.Client) mail.DomainManager {
	return resend.NewDomains(client, mail.DomainOptions{Token: drivertest.DomainToken})
}

func startDomains(t *testing.T, route drivertest.DomainRoute) (mail.DomainManager, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	return newDomains(client), httpclienttest.New(client).PreventStrayRequests().Stub(route.Pattern, route.Answer)
}

func TestDomains(t *testing.T) {
	t.Parallel()

	api := drivertest.ResendDomains()
	drivertest.RunDomains(t, api, newDomains)

	t.Run("creates the domain with tracking off", func(t *testing.T) {
		t.Parallel()
		manager, fake := startDomains(t, api.Create)

		_, err := manager.CreateDomain(t.Context(), "example.com")

		require.NoError(t, err)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			var body map[string]any
			require.NoError(t, json.Unmarshal(req.Body, &body))

			return body["name"] == "example.com" && body["open_tracking"] == false && body["click_tracking"] == false
		})
	})

	t.Run("places every kind of record and strips quotes", func(t *testing.T) {
		t.Parallel()
		manager, _ := startDomains(t, drivertest.DomainRoute{
			Pattern: "GET https://api.resend.com/domains/d1",
			Answer: httpclient.Response{Body: []byte(`{
				"id": "d1", "name": "example.com", "status": "pending", "records": [
					{"record": "Tracking", "name": "links", "type": "CNAME", "value": "links1.resend-dns.com"},
					{"record": "SPF", "name": "send", "type": "TXT", "value": "\"v=spf1 ~all\"", "status": "verified"},
					{"record": "Other", "name": "@", "type": "TXT", "value": "x"}
				]
			}`)},
		})

		domain, err := manager.GetDomain(t.Context(), "d1")

		require.NoError(t, err)
		assert.False(t, domain.Verified)
		assert.Equal(t, []mail.DomainRecord{
			{Purpose: mail.PurposeTracking, Type: "CNAME", Host: "links.example.com", Value: "links1.resend-dns.com"},
			{Purpose: mail.PurposeSPF, Type: "TXT", Host: "send.example.com", Value: "v=spf1 ~all", Valid: true},
			{Purpose: mail.PurposeOther, Type: "TXT", Host: "example.com", Value: "x"},
		}, domain.Records)
	})
}
