package identity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/saml/samltest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/sso"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

const metadataURL = "https://idp.example/metadata"

func TestSSOUpkeep(t *testing.T) {
	t.Parallel()

	t.Run("a rolled certificate is picked up from the metadata address", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		rolled, err := samltest.New(c.idp.EntityID, c.idp.SSOURL, c.tc.Clock.Now())
		require.NoError(t, err)
		in(c.tc).PatchJSON(connectionsPath+"/"+c.id, testkit.JSON{
			"config": testkit.JSON{
				"idp_entity_id": c.idp.EntityID, "idp_sso_url": c.idp.SSOURL, "idp_metadata_url": metadataURL,
				"idp_certificates": []string{c.idp.CertificatePEM()},
				"attribute_map":    testkit.JSON{"email": "email"},
			},
		}).AssertOK()
		httpclienttest.New(c.tc.App.HTTP).Stub(metadataURL, httpclient.Response{Body: rolled.Metadata()})

		refresh(t, c)

		got := in(c.tc).GetJSON(connectionsPath + "/" + c.id).AssertOK()
		got.AssertJSONPath("data.metadata_error", nil)
		assert.Equal(t, rolled.CertificatePEM(), got.String("data.config.idp_certificates.0"))
	})

	t.Run("a failed refresh keeps the old settings and tells the owners why", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		in(c.tc).PatchJSON(connectionsPath+"/"+c.id, testkit.JSON{
			"config": testkit.JSON{
				"idp_entity_id": c.idp.EntityID, "idp_sso_url": c.idp.SSOURL, "idp_metadata_url": metadataURL,
				"idp_certificates": []string{c.idp.CertificatePEM()},
			},
		}).AssertOK()
		httpclienttest.New(c.tc.App.HTTP).Stub(metadataURL, httpclient.Response{Status: 500})

		refresh(t, c)

		got := in(c.tc).GetJSON(connectionsPath + "/" + c.id).AssertOK()
		assert.Contains(t, got.String("data.metadata_error"), "500")
		assert.Equal(t, c.idp.CertificatePEM(), got.String("data.config.idp_certificates.0"))
	})

	t.Run("owners hear thirty days before the identity provider certificate expires", func(t *testing.T) {
		t.Parallel()
		c := newCompany(t, true)
		fake := c.tc.Mail()
		warn := func() {
			t.Helper()
			require.NoError(t, (&jobs.WarnExpiringSSOCertificates{
				DB: c.tc.App.DB, Clock: c.tc.Clock, Mail: c.tc.App.Mail, Links: c.tc.App.Links,
			}).Handle(t.Context(), &queue.Job[jobs.WarnExpiringSSOCertificatesArgs]{}))
			c.tc.Drain()
		}

		warn()
		sent, _ := tests.Sent[appmail.SSOCertificateExpiringMail](c.tc, fake)
		assert.Empty(t, sent)

		c.tc.Clock.Advance(c.idp.Certificate().NotAfter.Add(-30 * 24 * time.Hour).Sub(c.tc.Clock.Now()))
		warn()
		sent, _ = tests.Sent[appmail.SSOCertificateExpiringMail](c.tc, fake)
		require.Len(t, sent, 1)
		assert.Equal(t, "Okta", sent[0].Connection)
	})
}

/*
refresh runs the daily metadata refresh.
*/
func refresh(t *testing.T, c company) {
	t.Helper()

	require.NoError(t, (&jobs.RefreshSSOMetadata{
		DB: c.tc.App.DB, Refresh: &sso.RefreshMetadata{DB: c.tc.App.DB, HTTP: c.tc.App.HTTP, Clock: c.tc.Clock},
	}).Handle(t.Context(), &queue.Job[jobs.RefreshSSOMetadataArgs]{}))
}
