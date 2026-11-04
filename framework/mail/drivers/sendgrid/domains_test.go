package sendgrid_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/sendgrid"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

func newDomains(client *httpclient.Client) mail.DomainManager {
	return sendgrid.NewDomains(client, mail.DomainOptions{Token: drivertest.DomainToken})
}

func TestDomains(t *testing.T) {
	t.Parallel()

	api := drivertest.SendGridDomains()
	drivertest.RunDomains(t, api, newDomains)

	t.Run("asks for managed records and turns account tracking off", func(t *testing.T) {
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
			var body map[string]any
			require.NoError(t, json.Unmarshal(req.Body, &body))

			return body["domain"] == "example.com" && body["automatic_security"] == true
		})
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			var body map[string]any
			require.NoError(t, json.Unmarshal(req.Body, &body))

			return req.Method == http.MethodPatch && body["enabled"] == false
		})
	})

	t.Run("uses the eu endpoint", func(t *testing.T) {
		t.Parallel()
		client, err := httpclient.New(httpclient.Config{AllowFakes: true})
		require.NoError(t, err)

		httpclienttest.New(client).PreventStrayRequests().Stub(
			"GET https://api.eu.sendgrid.com/v3/whitelabel/domains/45373692", api.Read.Answer,
		)
		manager := sendgrid.NewDomains(client, mail.DomainOptions{
			Token: drivertest.DomainToken, Endpoint: "https://api.eu.sendgrid.com",
		})

		domain, err := manager.GetDomain(t.Context(), "45373692")

		require.NoError(t, err)
		assert.Equal(t, api.Records, domain.Records)
	})
}
