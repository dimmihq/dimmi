/*
|--------------------------------------------------------------------------
| Email Channel
|--------------------------------------------------------------------------
|
| The email channel's actions, its adapter and its job. Received mail is
| kept on the raw disk, the configured one or the default, and replies
| leave through the customer-stream mailer, whose driver names the
| transport every sent email records. Stamps are keyed by a key derived
| from the application key, so only this install can make them.
|
| Every new workspace gets its first forwarding address while it is
| created, once the install receives mail.
|
*/

package providers

import (
	"cmp"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/email"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
stampsKeyPurpose names the key that signs the email channel's stamps.
*/
const stampsKeyPurpose = "email-stamps"

/*
bytesPerMB converts the memory budget's setting to bytes.
*/
const bytesPerMB = 1 << 20

/*
registerEmail registers the email channel's workspace defaults. It runs
after the default inbox is registered, since the first address lands
in that inbox.
*/
func registerEmail(a *app.App, fa *foundation.Application) {
	a.WorkspaceInitializers.Register(&email.EnsureDefaultEmailAddress{
		Clock: fa.Clock, Config: a.Config.Channels.Email,
	})
}

/*
bootEmail builds the email channel's actions and adapter and registers
its job.
*/
func bootEmail(a *app.App) error {
	acts := a.Actions
	cfg := a.Config.Channels.Email
	stamps := inbound.NewStamps(a.DerivedKey(stampsKeyPurpose))
	transport := customerTransport(a)
	diskName := cmp.Or(cfg.Inbound.RawDisk, a.Application.Config.Filesystems.Default)

	disk, err := a.Storage.Disk(diskName)
	if err != nil {
		return fmt.Errorf("email disk: %w", err)
	}

	acts.CreateEmailAddress = &email.CreateEmailAddress{DB: a.DB, Clock: a.Clock, Config: cfg}
	acts.ListEmailAddresses = &email.ListEmailAddresses{DB: a.DB}
	acts.ShowEmailAddress = &email.ShowEmailAddress{DB: a.DB}
	acts.UpdateEmailAddress = &email.UpdateEmailAddress{DB: a.DB, Clock: a.Clock}
	acts.DeleteEmailAddress = &email.DeleteEmailAddress{DB: a.DB, Clock: a.Clock}
	acts.ListInboundEmails = &email.ListInboundEmails{DB: a.DB}
	acts.SendEmailAddressTest = &email.SendEmailAddressTest{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, Config: cfg, Stamps: stamps, Transport: transport,
	}
	acts.ReceiveInboundEmail = &email.ReceiveInboundEmail{
		DB: a.DB, Clock: a.Clock, Queue: a.Queue, Disk: disk, DiskName: diskName, Config: cfg,
	}
	if err := bootInboundSources(a); err != nil {
		return err
	}

	acts.ProcessInboundEmail = &email.ProcessInboundEmail{
		DB: a.DB, Clock: a.Clock, Storage: a.Storage, DNS: a.DNS, Limiter: a.Limiter, Config: cfg, Stamps: stamps,
		Receiver: &conversations.ReceiveInboundMessage{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName,
			ReopenWindow: cfg.Inbound.ReopenWindow,
		},
	}
	acts.SendEmailMessage = &email.SendEmailMessage{
		DB: a.DB, Clock: a.Clock, Mail: a.Mail, IDs: a.IDs, Config: cfg, Stamps: stamps, Transport: transport,
	}
	a.EmailChannel = &inbound.Channel{Sender: acts.SendEmailMessage, Transport: transport}
	if err := a.Channels.Register(a.EmailChannel); err != nil {
		return err
	}

	return queue.Register(a.Queue, &jobs.ProcessInboundEmail{Processor: acts.ProcessInboundEmail})
}

/*
bootInboundSources builds the actions behind the provider webhooks and
the operator's checks of them, after the receiving action exists.
*/
func bootInboundSources(a *app.App) error {
	acts := a.Actions
	cfg := a.Config.Channels.Email

	acts.InboundHealth = &email.InboundHealth{Cache: a.Cache, Log: a.Log}
	acts.InboundBudget = mailbox.NewBudget(cfg.Inbound.ParseMemoryBudgetMB * bytesPerMB)
	acts.BackfillResend = &email.BackfillResend{Sources: a.Mailbox, Receive: acts.ReceiveInboundEmail}
	acts.InboundDoctor = &email.InboundDoctor{
		DB: a.DB, Clock: a.Clock, DNS: a.DNS, Sources: a.Mailbox, Health: acts.InboundHealth, Config: cfg,
		AppURL: a.Application.Config.App.URL,
	}

	return checkInboundSources(a)
}

/*
checkInboundSources builds every enabled source, so an install with a
missing secret refuses to start and names the key, rather than failing on
the first mail. The test source takes no options; SES reads a bucket and
is checked when its route is wired.
*/
func checkInboundSources(a *app.App) error {
	for _, source := range a.Config.Channels.Email.Inbound.Drivers {
		if source == appconfig.InboundSourceTest || source == appconfig.InboundSourceSES {
			continue
		}

		if _, err := a.Mailbox.Receiver(source); err != nil {
			return fmt.Errorf("channels.email.inbound.drivers %s: %w", source, err)
		}
	}

	return nil
}

/*
customerTransport returns the driver of the customer-stream mailer,
which falls back to the default mailer, as the transport sent email
records. A driver the records do not know counts as the log driver.
*/
func customerTransport(a *app.App) string {
	mail := a.Application.Config.Mail

	section, ok := mail.Mailers[a.Config.Channels.Email.Mailer]
	if !ok {
		section = mail.Mailers[mail.Default]
	}

	switch driver := section.String("driver"); driver {
	case "ses", "smtp", "postmark", "resend", "mailgun", "sendgrid":
		return driver
	}

	return "log"
}
