package mailgun_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

func newDomains(client *httpclient.Client) mail.DomainManager {
	return mailgun.NewDomains(client, mail.DomainOptions{Token: drivertest.DomainToken})
}

func TestDomains(t *testing.T) {
	t.Parallel()

	api := drivertest.MailgunDomains()
	drivertest.RunDomains(t, api, newDomains)

	t.Run("posts the name and sends tracking off as forms", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)

		fake := httpclienttest.New(client).PreventStrayRequests().Stub(api.Create.Pattern, api.Create.Answer)
		for _, route := range api.Tracking {
			fake.Stub(route.Pattern, route.Answer)
		}

		_, err = newDomains(client).CreateDomain(t.Context(), "example.com")
		require.NoError(t, err)

		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Method == http.MethodPost && string(req.Body) == "name=example.com" &&
				req.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
		})

		for _, kind := range []string{"click", "open"} {
			fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
				return req.URL.Path == "/v3/domains/example.com/tracking/"+kind && string(req.Body) == "active=no" &&
					req.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
			})
		}
	})

	t.Run("places every kind of record and uses the eu endpoint", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)

		httpclienttest.New(client).PreventStrayRequests().Stub(
			"GET https://api.eu.mailgun.net/v4/domains/example.com",
			httpclient.Response{Body: []byte(`{
				"domain": {"name": "example.com", "state": "disabled"},
				"sending_dns_records": [
					{"record_type": "CNAME", "name": "pdk1._domainkey.example.com.", "value": "k.mailgun.org"},
					{"record_type": "MX", "name": "example.com", "value": "mx.mailgun.org"},
					{"record_type": "TXT", "name": "example.com", "value": "other"}
				]
			}`)},
		)
		manager := mailgun.NewDomains(client, mail.DomainOptions{
			Token: drivertest.DomainToken, Endpoint: "https://api.eu.mailgun.net",
		})

		domain, err := manager.GetDomain(t.Context(), "example.com")

		require.NoError(t, err)
		assert.False(t, domain.Verified)
		assert.Equal(t, []mail.DomainRecord{
			{Purpose: mail.PurposeDKIM, Type: "CNAME", Host: "pdk1._domainkey.example.com", Value: "k.mailgun.org"},
			{Purpose: mail.PurposeMX, Type: "MX", Host: "example.com", Value: "mx.mailgun.org"},
			{Purpose: mail.PurposeOther, Type: "TXT", Host: "example.com", Value: "other"},
		}, domain.Records)
	})
}
