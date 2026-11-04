/*
|--------------------------------------------------------------------------
| Domain Manager Selection
|--------------------------------------------------------------------------
|
| A domain credential is optional, and only some drivers have a domain
| API. Application code holds a driver name from configuration, so it asks
| here and treats ErrDomainsUnsupported as the sign that domains are set
| up by hand at the provider.
|
*/

package domains

import (
	"fmt"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mail/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mail/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mail/drivers/sendgrid"
)

/*
For returns the domain manager of the named driver. It returns
mail.ErrDomainsUnsupported for a driver without a domain API and when the
options hold no token.
*/
func For(client *httpclient.Client, driver string, opts mail.DomainOptions) (mail.DomainManager, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("%s: no token: %w", driver, mail.ErrDomainsUnsupported)
	}

	switch driver {
	case "postmark":
		return postmark.NewDomains(client, opts), nil
	case "resend":
		return resend.NewDomains(client, opts), nil
	case "mailgun":
		return mailgun.NewDomains(client, opts), nil
	case "sendgrid":
		return sendgrid.NewDomains(client, opts), nil
	default:
		return nil, fmt.Errorf("%s: %w", driver, mail.ErrDomainsUnsupported)
	}
}
