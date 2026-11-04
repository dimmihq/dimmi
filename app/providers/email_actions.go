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

	fconfig "github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/email"
	inbound "github.com/dimmihq/dimmi/app/integrations/email"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
stampsKeyPurpose names the key that signs the email channel's stamps.
*/
const stampsKeyPurpose = "email-stamps"

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
customerSection returns the configuration of the customer-stream
mailer, the default mailer's when it has none of its own.
*/
func customerSection(a *app.App) fconfig.Section {
	mail := a.Application.Config.Mail

	section, ok := mail.Mailers[a.Config.Channels.Email.Mailer]
	if !ok {
		section = mail.Mailers[mail.Default]
	}

	return section
}

/*
customerTransport returns the driver of the customer-stream mailer,
which falls back to the default mailer, as the transport sent email
records. A driver the records do not know counts as the log driver.
*/
func customerTransport(a *app.App) string {
	switch driver := customerSection(a).String("driver"); driver {
	case "ses", "smtp", "postmark", "resend", "mailgun", "sendgrid":
		return driver
	}

	return "log"
}
