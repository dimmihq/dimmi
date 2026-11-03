/*
|--------------------------------------------------------------------------
| Channel Settings
|--------------------------------------------------------------------------
|
| How email reaches and leaves the install. Mail arrives at forwarding
| addresses on one inbound domain, through a receiving provider, never
| through a mail server of the install's own: without an inbound domain
| no address can receive. Replies leave through one named mailer, the
| customer stream, kept apart from the install's own account mail.
|
| The cloud receives on its own inbound domain; a self-hosted install
| sets one only once it has a provider receiving for it. The test source
| takes raw messages over HTTP for end-to-end runs and is refused
| without its shared secret.
|
*/

package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

/*
Inbound sources an install may enable.
*/
const (
	InboundSourceSES  = "ses"
	InboundSourceTest = "test"
)

/*
cloudInboundDomain is the domain the cloud receives mail on.
*/
const cloudInboundDomain = "inbound.trydimmi.com"

/*
ChannelsConfig configures the conversation channels.
*/
type ChannelsConfig struct {
	Email EmailConfig `koanf:"email"`
}

/*
EmailConfig configures the email channel.

InboundDomain is the domain forwarding addresses live on, empty when the
install receives no mail. DefaultSenderDomain is the domain replies come
from while a workspace has no verified domain of its own; it defaults to
the inbound domain. Mailer names the mailer of the customer stream; a
name the mail configuration lacks falls back to its default mailer.
*/
type EmailConfig struct {
	InboundDomain       string              `koanf:"inbound_domain"`
	DefaultSenderDomain string              `koanf:"default_sender_domain"`
	Mailer              string              `default:"customer"        koanf:"mailer"`
	Inbound             EmailInboundConfig  `koanf:"inbound"`
	Outbound            EmailOutboundConfig `koanf:"outbound"`
}

/*
EmailInboundConfig configures received mail.

Drivers lists the enabled inbound sources; TestSecret is the bearer
secret the test source requires. MaxMessageBytes is the largest message
parsed whole, MaxAttachments and MaxAttachmentBytes bound the files kept
from one message. RawDisk names the disk raw messages are kept on, the
default disk when empty. SenderRate mails from one sender within
SenderRateWindow are kept but never answered automatically;
SenderQuarantine within SenderQuarantineWindow are set aside as spam.
ReopenWindow is how long after closing a reply still reopens a
conversation rather than starting a new one.
*/
type EmailInboundConfig struct {
	Drivers                []string      `koanf:"drivers"`
	TestSecret             string        `koanf:"test_secret"              secret:"true"`
	MaxMessageBytes        int64         `default:"31457280"               koanf:"max_message_bytes"`
	MaxAttachments         int           `default:"50"                     koanf:"max_attachments"`
	MaxAttachmentBytes     int64         `default:"26214400"               koanf:"max_attachment_bytes"`
	RawDisk                string        `koanf:"raw_disk"`
	SenderRate             int           `default:"20"                     koanf:"sender_rate"`
	SenderRateWindow       time.Duration `default:"10m"                    koanf:"sender_rate_window"`
	SenderQuarantine       int           `default:"100"                    koanf:"sender_quarantine"`
	SenderQuarantineWindow time.Duration `default:"1h"                     koanf:"sender_quarantine_window"`
	ReopenWindow           time.Duration `default:"720h"                   koanf:"reopen_window"`
}

/*
EmailOutboundConfig configures sent mail. ReferencesMax is how many
earlier ids a reply's References header carries at most.
*/
type EmailOutboundConfig struct {
	ReferencesMax int `default:"20" koanf:"references_max"`
}

/*
Receives reports whether the install receives mail at all.
*/
func (c EmailConfig) Receives() bool {
	return c.InboundDomain != ""
}

/*
SourceEnabled reports whether the inbound source is enabled.
*/
func (c EmailConfig) SourceEnabled(source string) bool {
	return slices.Contains(c.Inbound.Drivers, source)
}

/*
settle fills the defaults that depend on the edition: the cloud
receives on its own domain through SES.
*/
func (c *ChannelsConfig) settle(edition Edition) {
	email := &c.Email
	if edition == EditionCloud && email.InboundDomain == "" {
		email.InboundDomain = cloudInboundDomain
	}

	if edition == EditionCloud && len(email.Inbound.Drivers) == 0 {
		email.Inbound.Drivers = []string{InboundSourceSES}
	}

	email.InboundDomain = strings.ToLower(strings.TrimSpace(email.InboundDomain))
	if email.DefaultSenderDomain == "" {
		email.DefaultSenderDomain = email.InboundDomain
	}
}

/*
check reports the first channel setting that cannot work.
*/
func (c *ChannelsConfig) check() error {
	email := c.Email
	unknown := slices.ContainsFunc(email.Inbound.Drivers, func(s string) bool {
		return s != InboundSourceSES && s != InboundSourceTest
	})

	problems := []struct {
		found bool
		key   string
	}{
		{email.InboundDomain != "" && !hostname(email.InboundDomain), "channels.email.inbound_domain"},
		{unknown, "channels.email.inbound.drivers"},
		{
			email.SourceEnabled(InboundSourceTest) && email.Inbound.TestSecret == "",
			"channels.email.inbound.test_secret",
		},
		{email.Inbound.MaxMessageBytes <= 0, "channels.email.inbound.max_message_bytes"},
		{email.Inbound.MaxAttachments < 0, "channels.email.inbound.max_attachments"},
		{email.Inbound.SenderRate <= 0 || email.Inbound.SenderRateWindow <= 0, "channels.email.inbound.sender_rate"},
		{
			email.Inbound.SenderQuarantine <= 0 || email.Inbound.SenderQuarantineWindow <= 0,
			"channels.email.inbound.sender_quarantine",
		},
		{email.Outbound.ReferencesMax < 1, "channels.email.outbound.references_max"},
	}

	for _, p := range problems {
		if p.found {
			return fmt.Errorf("%w: %s", ErrInvalid, p.key)
		}
	}

	return nil
}

/*
hostname reports whether the name looks like a domain mail can be
addressed to: at least two dot-separated labels of letters, digits and
dashes.
*/
func hostname(name string) bool {
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}

		if strings.ContainsFunc(label, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
		}) {
			return false
		}
	}

	return true
}
