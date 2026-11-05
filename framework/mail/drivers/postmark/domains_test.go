package postmark_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
)

func newDomains(client *httpclient.Client) mail.DomainManager {
	return postmark.NewDomains(client, mail.DomainOptions{Token: drivertest.DomainToken})
}

func startDomains(t *testing.T, route drivertest.DomainRoute) (mail.DomainManager, *httpclienttest.Fake) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)

	return newDomains(client), httpclienttest.New(client).PreventStrayRequests().Stub(route.Pattern, route.Answer)
}

func TestDomains(t *testing.T) {
	t.Parallel()

	api := drivertest.PostmarkDomains()
	drivertest.RunDomains(t, api, newDomains)

	t.Run("posts the domain name", func(t *testing.T) {
		t.Parallel()
		manager, fake := startDomains(t, api.Create)

		_, err := manager.CreateDomain(t.Context(), "example.com")

		require.NoError(t, err)
		fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			var body map[string]string
			require.NoError(t, json.Unmarshal(req.Body, &body))

			return body["Name"] == "example.com"
		})
	})

	t.Run("reports both keys during a rotation", func(t *testing.T) {
		t.Parallel()
		manager, _ := startDomains(t, drivertest.DomainRoute{
			Pattern: api.Read.Pattern,
			Answer: httpclient.Response{Body: []byte(`{
				"ID": 36736, "DKIMVerified": true,
				"DKIMHost": "old._domainkey.example.com", "DKIMTextValue": "old-key",
				"DKIMPendingHost": "new._domainkey.example.com", "DKIMPendingTextValue": "new-key",
				"ReturnPathDomain": "pm-bounces.example.com", "ReturnPathDomainVerified": true,
				"ReturnPathDomainCNAMEValue": "pm.mtasv.net"
			}`)},
		})

		domain, err := manager.GetDomain(t.Context(), "36736")

		require.NoError(t, err)
		assert.True(t, domain.Verified)
		assert.Equal(t, []mail.DomainRecord{
			{Purpose: mail.PurposeDKIM, Type: "TXT", Host: "old._domainkey.example.com", Value: "old-key", Valid: true},
			{Purpose: mail.PurposeDKIM, Type: "TXT", Host: "new._domainkey.example.com", Value: "new-key"},
			{
				Purpose: mail.PurposeReturnPath, Type: "CNAME", Host: "pm-bounces.example.com",
				Value: "pm.mtasv.net", Valid: true,
			},
		}, domain.Records)
	})

	t.Run("treats a plain 404 as a missing domain", func(t *testing.T) {
		t.Parallel()
		manager, _ := startDomains(t, drivertest.DomainRoute{
			Pattern: api.Read.Pattern, Answer: httpclient.Response{Status: http.StatusNotFound},
		})

		_, err := manager.GetDomain(t.Context(), "36736")

		require.ErrorIs(t, err, mail.ErrDomainNotFound)
	})
}
