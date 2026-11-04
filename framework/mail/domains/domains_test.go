package domains_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/domains"
	"github.com/dimmihq/dimmi/framework/mail/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mail/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mail/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mail/drivers/sendgrid"
)

func TestFor(t *testing.T) {
	t.Parallel()

	client, err := httpclient.New(httpclient.Config{})
	require.NoError(t, err)

	token := mail.DomainOptions{Token: "secret"}

	t.Run("returns the manager of each driver with a domain api", func(t *testing.T) {
		t.Parallel()

		managers := map[string]mail.DomainManager{
			"postmark": postmark.NewDomains(client, token),
			"resend":   resend.NewDomains(client, token),
			"mailgun":  mailgun.NewDomains(client, token),
			"sendgrid": sendgrid.NewDomains(client, token),
		}

		for driver, want := range managers {
			got, err := domains.For(client, driver, token)
			require.NoError(t, err, driver)
			assert.IsType(t, want, got, driver)
		}
	})

	t.Run("refuses a driver without a domain api", func(t *testing.T) {
		t.Parallel()

		for _, driver := range []string{"smtp", "ses", "log", ""} {
			_, err := domains.For(client, driver, token)
			require.ErrorIs(t, err, mail.ErrDomainsUnsupported, driver)
		}
	})

	t.Run("refuses a driver without a token", func(t *testing.T) {
		t.Parallel()

		_, err := domains.For(client, "postmark", mail.DomainOptions{})

		require.ErrorIs(t, err, mail.ErrDomainsUnsupported)
	})
}
