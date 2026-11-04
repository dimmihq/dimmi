/*
|--------------------------------------------------------------------------
| Email Sending Settings
|--------------------------------------------------------------------------
|
| How mail leaves for customers: which domains the operator set up at the
| instance's provider, how a domain is checked, what the sending provider
| needs to report back, and the limits and pauses that protect the
| install's reputation. Every webhook secret stays out of logs and out of
| config:show.
|
| Limits are off on a self-hosted install until its operator sets them, and
| the cloud gets its own reputation defaults, because a bad sender there
| hurts every other customer on the shared pools.
|
*/

package config

import (
	"fmt"
	"time"
)

/*
Cloud reputation defaults: the rates that pause a workspace, and the
resolvers a domain check asks, which bypass any local cache.
*/
const (
	cloudBouncePauseRate    = 0.05
	cloudComplaintPauseRate = 0.001
)

/*
Recipient protections every install has: relayed mail from an unproven
sender per address a day, and automated mail per recipient an hour.
*/
const (
	defaultRelayPerAddressDaily        = 3
	defaultAutomatedPerRecipientHourly = 10
)

/*
EmailSendingConfig configures sent customer mail.

ProviderVerifiedDomains lists the domains the operator set up at a provider
by hand; a parent covers its subdomains. Resolvers name the DNS servers a
domain check asks, host:port, the install's own when empty. SelectorPrefix
starts the key names of domains a relay's mail is signed for. MaxAttachmentBytes
is the largest total sent as attachments, the rest going as links that live
for AttachmentLinkTTL.
*/
type EmailSendingConfig struct {
	ProviderVerifiedDomains []string            `koanf:"provider_verified_domains"`
	Resolvers               []string            `koanf:"resolvers"`
	SelectorPrefix          string              `default:"dimmi"           koanf:"selector_prefix"`
	RelaySPFInclude         string              `koanf:"relay_spf_include"`
	MaxAttachmentBytes      int64               `default:"20971520"        koanf:"max_attachment_bytes"`
	AttachmentLinkTTL       time.Duration       `default:"168h"            koanf:"attachment_link_ttl"`
	SES                     EmailSESConfig      `koanf:"ses"`
	Postmark                EmailProviderConfig `koanf:"postmark"`
	Resend                  EmailProviderConfig `koanf:"resend"`
	Mailgun                 EmailProviderConfig `koanf:"mailgun"`
	SendGrid                EmailProviderConfig `koanf:"sendgrid"`
	Limits                  EmailLimitsConfig   `koanf:"limits"`
	Reputation              EmailReputation     `koanf:"reputation"`
	DomainConnect           DomainConnectConfig `koanf:"domain_connect"`
	RoundTrip               RoundTripConfig     `koanf:"round_trip"`
	WidgetOfflineEnabled    bool                `default:"true"            koanf:"widget_offline_enabled"`
	OpsForwardTo            string              `koanf:"ops_forward_to"`
}

/*
EmailSESConfig configures sending through SES. AccountID completes the
resource names a tenant holds; without it identities are not tied to
tenants. PauseAll stops the customer stream on the whole install.
*/
type EmailSESConfig struct {
	EventsTopicARN     string `koanf:"events_topic_arn"`
	SystemTopicARN     string `koanf:"system_events_topic_arn"`
	ConfigurationSet   string `default:"dimmi-customer" koanf:"configuration_set"`
	TenantPerWorkspace bool   `koanf:"tenant_per_workspace"`
	MailFromSubdomain  string `default:"bounce"         koanf:"mail_from_subdomain"`
	AccountID          string `koanf:"account_id"`
	PauseAll           bool   `koanf:"pause_all"`
}

/*
EmailProviderConfig configures one mail provider's event webhook and domain
API. WebhookSecret is what proves its events: a basic auth password, a signing
secret or a public key, by provider. Endpoint replaces its API base, for a
region of its own. MessageStream names Postmark's customer stream.
*/
type EmailProviderConfig struct {
	WebhookSecret  string `koanf:"webhook_secret"   secret:"true"`
	DomainAPIToken string `koanf:"domain_api_token" secret:"true"`
	Endpoint       string `koanf:"endpoint"`
	MessageStream  string `koanf:"message_stream"`
}

/*
settle fills what depends on the edition.
*/
func (c *EmailSendingConfig) settle(edition Edition) {
	if c.Limits.UnverifiedRelayPerAddressDay <= 0 {
		c.Limits.UnverifiedRelayPerAddressDay = defaultRelayPerAddressDaily
	}

	if c.Limits.AutomatedPerRecipientHourly <= 0 {
		c.Limits.AutomatedPerRecipientHourly = defaultAutomatedPerRecipientHourly
	}

	if edition != EditionCloud {
		return
	}

	c.SES.TenantPerWorkspace = true
	if len(c.Resolvers) == 0 {
		c.Resolvers = []string{"1.1.1.1:53", "8.8.8.8:53"}
	}

	if c.Reputation.BouncePauseRate == 0 {
		c.Reputation.BouncePauseRate = cloudBouncePauseRate
	}

	if c.Reputation.ComplaintPauseRate == 0 {
		c.Reputation.ComplaintPauseRate = cloudComplaintPauseRate
	}
}

/*
check reports the first sending setting that cannot work.
*/
func (c *EmailSendingConfig) check() error {
	connect, trip := c.DomainConnect, c.RoundTrip
	problems := []struct {
		found bool
		key   string
	}{
		{c.MaxAttachmentBytes <= 0, "channels.email.sending.max_attachment_bytes"},
		{c.AttachmentLinkTTL <= 0, "channels.email.sending.attachment_link_ttl"},
		{c.Reputation.BouncePauseRate < 0 || c.Reputation.BouncePauseRate > 1, "channels.email.sending.reputation"},
		{connect.Enabled && connect.PrivateKey == "", "channels.email.sending.domain_connect.private_key"},
		{trip.Enabled && trip.Workspace == "", "channels.email.sending.round_trip.workspace"},
	}

	for _, p := range problems {
		if p.found {
			return fmt.Errorf("%w: %s", ErrInvalid, p.key)
		}
	}

	return nil
}
