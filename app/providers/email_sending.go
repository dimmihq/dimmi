/*
|--------------------------------------------------------------------------
| Email Sending
|--------------------------------------------------------------------------
|
| Sending domains are set up with whoever sends the install's customer
| mail. SES sets them up through its identities, in the region and
| account of the customer-stream mailer; a provider with a domain API
| through that API, once the operator gave it a token; any other driver
| is set up by hand and listed in configuration. Due checks are queued
| every minute.
|
*/

package providers

import (
	"context"
	"errors"
	"sync"

	"github.com/dimmihq/dimmi/framework/aws"
	"github.com/dimmihq/dimmi/framework/mail"
	maildomains "github.com/dimmihq/dimmi/framework/mail/domains"
	"github.com/dimmihq/dimmi/framework/mail/drivers/ses"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/email"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/config"
)

/*
bootEmailSending builds the sending domain actions, registers their jobs
and broadcasts their changes.
*/
func bootEmailSending(a *app.App) error {
	cfg := a.Config.Channels.Email
	domains := &email.Domains{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Keys: a.Keys, DNS: a.DNS, Driver: customerTransport(a),
		Mailer: customerSection(a), Config: cfg, Edition: a.Config.App.Edition,
	}

	domains.Provider, domains.Region = domainProvider(a, domains.Driver, cfg.Sending)
	a.EmailDomains = domains

	(&appbroadcasting.EmailSending{Server: a.Broadcast}).Listen(a.Events)

	return errors.Join(
		queue.Register(a.Queue, &jobs.CheckEmailDomain{Checker: &email.CheckEmailDomain{Domains: domains}}),
		queue.Register(a.Queue, &jobs.ScheduleEmailDomainChecks{DB: a.DB, Queue: a.Queue, Clock: a.Clock}),
	)
}

/*
domainProvider returns the provider side of domains for the driver, and
the SES region return paths point at. A provider without a domain API,
or without a token for it, has none.
*/
func domainProvider(a *app.App, driver string, cfg config.EmailSendingConfig) (email.DomainProvider, string) {
	if driver == "ses" {
		section := customerSection(a)
		region := section.String("region")

		return &email.SESDomains{
			DB: a.DB, Clock: a.Clock, Connect: sesConnector(a), TenantPerWorkspace: cfg.SES.TenantPerWorkspace,
			Account: cfg.SES.AccountID, Region: region,
		}, region
	}

	opts := map[string]config.EmailProviderConfig{
		"postmark": cfg.Postmark, "resend": cfg.Resend, "mailgun": cfg.Mailgun, "sendgrid": cfg.SendGrid,
	}[driver]

	manager, err := maildomains.For(a.HTTP, driver, mail.DomainOptions{
		Token: opts.DomainAPIToken, Endpoint: opts.Endpoint,
	})
	if err != nil {
		return nil, ""
	}

	return &email.APIDomains{Manager: manager}, ""
}

/*
sesConnector builds the SES identities API once, on first use, from the
customer-stream mailer's AWS account.
*/
func sesConnector(a *app.App) func(ctx context.Context) (email.SESIdentities, error) {
	var (
		mu         sync.Mutex
		identities *ses.Identities
	)

	return func(ctx context.Context) (email.SESIdentities, error) {
		mu.Lock()
		defer mu.Unlock()

		if identities != nil {
			return identities, nil
		}

		cfg, err := aws.Config(context.WithoutCancel(ctx), customerSection(a), a.HTTP)
		if err != nil {
			return nil, err
		}

		identities = ses.NewIdentities(cfg)

		return identities, nil
	}
}
