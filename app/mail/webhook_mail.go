/*
|--------------------------------------------------------------------------
| Webhook Mail
|--------------------------------------------------------------------------
|
| An endpoint sends the workspace's data to a server outside it, so its
| owners hear the moment one is added, with who added it, and nobody
| finds out weeks later. Owners and admins also hear when an endpoint
| has failed for a day and when it is turned off, with the link to its
| delivery log.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
WebhookEndpointAddedMail tells an owner that Actor added an endpoint at
Host to Workspace, and links the webhook settings.
*/
type WebhookEndpointAddedMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Host        string `json:"host"`
	Actor       string `json:"actor"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WebhookEndpointAddedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the host.
*/
func (m WebhookEndpointAddedMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.WebhookEndpointAdded(m.Name, m.Workspace, m.Host, m.Actor, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.webhook_endpoint_added.subject", i18n.Args{"host": m.Host})

	return c, nil
}

/*
WebhookEndpointFailingMail tells an owner or admin that the endpoint at
Host has failed for a day, with LastError, the last error class.
*/
type WebhookEndpointFailingMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Host        string `json:"host"`
	LastError   string `json:"last_error"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WebhookEndpointFailingMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the host.
*/
func (m WebhookEndpointFailingMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.WebhookEndpointFailing(m.Name, m.Workspace, m.Host, m.LastError, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.webhook_endpoint_failing.subject", i18n.Args{"host": m.Host})

	return c, nil
}

/*
WebhookEndpointDisabledMail tells an owner or admin that the endpoint at
Host was turned off. Reason is auto_failure or gone.
*/
type WebhookEndpointDisabledMail struct {
	Name        string `json:"name"`
	Workspace   string `json:"workspace"`
	Host        string `json:"host"`
	Reason      string `json:"reason"`
	SettingsURL string `json:"settings_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (WebhookEndpointDisabledMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail, its subject naming the host.
*/
func (m WebhookEndpointDisabledMail) Content(ctx context.Context) (mail.Content, error) {
	c := mail.View(accounts.WebhookEndpointDisabled(m.Name, m.Workspace, m.Host, m.Reason, m.SettingsURL))
	c.Subject = i18n.T(ctx, "mail.webhook_endpoint_disabled.subject", i18n.Args{"host": m.Host})

	return c, nil
}

/*
registerSecurityNotices lets the mail be queued that tells owners about
changes to who may sign in and where the workspace's data goes.
*/
func registerSecurityNotices(m *mail.Manager) {
	mail.Register[AccessPolicyChangedMail](m, "access_policy_changed")
	mail.Register[WebhookEndpointAddedMail](m, "webhook_endpoint_added")
	mail.Register[WebhookEndpointFailingMail](m, "webhook_endpoint_failing")
	mail.Register[WebhookEndpointDisabledMail](m, "webhook_endpoint_disabled")
}
