/*
|--------------------------------------------------------------------------
| Built-in Drivers
|--------------------------------------------------------------------------
|
| Every driver the framework ships is registered with its manager here,
| so configuration alone picks one and application code never registers
| a built-in. Drivers that call AWS build their SDK configuration the
| first time an instance needs it, so an installation that never uses
| AWS never loads credentials.
|
*/

package foundation

import (
	"context"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"

	"github.com/dimmihq/dimmi/framework/aws"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/caddyask"
	"github.com/dimmihq/dimmi/framework/customdomain/drivers/certmagic"
	tlsnone "github.com/dimmihq/dimmi/framework/customdomain/drivers/none"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/encryption/drivers/awskms"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/s3"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/argon2id"
	"github.com/dimmihq/dimmi/framework/hashing/drivers/bcrypt"
	langnone "github.com/dimmihq/dimmi/framework/i18n/drivers/none"
	"github.com/dimmihq/dimmi/framework/i18n/drivers/whatlang"
	maillog "github.com/dimmihq/dimmi/framework/mail/drivers/log"
	"github.com/dimmihq/dimmi/framework/mail/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mail/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mail/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mail/drivers/sendgrid"
	mailses "github.com/dimmihq/dimmi/framework/mail/drivers/ses"
	"github.com/dimmihq/dimmi/framework/mail/drivers/smtp"
	"github.com/dimmihq/dimmi/framework/mailbox"
	inboundcloudflare "github.com/dimmihq/dimmi/framework/mailbox/drivers/cloudflare"
	inboundmailgun "github.com/dimmihq/dimmi/framework/mailbox/drivers/mailgun"
	inboundpostmark "github.com/dimmihq/dimmi/framework/mailbox/drivers/postmark"
	inboundresend "github.com/dimmihq/dimmi/framework/mailbox/drivers/resend"
	inboundsendgrid "github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid"
	inboundses "github.com/dimmihq/dimmi/framework/mailbox/drivers/ses"
	"github.com/dimmihq/dimmi/framework/malware/drivers/clamav"
	"github.com/dimmihq/dimmi/framework/malware/drivers/guardduty"
	scannone "github.com/dimmihq/dimmi/framework/malware/drivers/none"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/notifications/channels/broadcast"
	notifydb "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	mailchannel "github.com/dimmihq/dimmi/framework/notifications/channels/mail"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/generic"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/hosted"
	"github.com/dimmihq/dimmi/framework/oauthclient/drivers/slack"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/github"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/google"
	"github.com/dimmihq/dimmi/framework/socialite/drivers/oidc"
)

/*
registerDrivers registers the drivers of storage, scanning, mail,
inbound mail, notifications, hashing and key encryption.
*/
func (a *Application) registerDrivers(ctx context.Context) {
	awsConfig := a.lazyAWS(ctx)

	a.Storage.Extend("local", local.Factory)
	a.Storage.Extend("s3", s3.Factory(context.WithoutCancel(ctx)))

	a.Scanner.Extend("none", scannone.Factory)
	a.Scanner.Extend("clamav", clamav.Factory)
	a.Scanner.Extend("guardduty", withAWS(awsConfig, guardduty.Factory))

	a.Mail.Extend("log", maillog.Factory(a.Log, a.Clock))
	a.Mail.Extend("smtp", smtp.Factory(a.Clock))
	a.Mail.Extend("ses", mailses.Factory(context.WithoutCancel(ctx), a.HTTP, a.Clock))
	a.Mail.Extend("postmark", postmark.Factory(a.HTTP))
	a.Mail.Extend("resend", resend.Factory(a.HTTP))
	a.Mail.Extend("mailgun", mailgun.Factory(a.HTTP, a.Clock))
	a.Mail.Extend("sendgrid", sendgrid.Factory(a.HTTP))

	replays := mailbox.NewReplays(a.Cache)
	a.Mailbox.Extend("ses", inboundses.Factory(a.HTTP, a.Storage))
	a.Mailbox.Extend("postmark", inboundpostmark.Factory(a.Clock))
	a.Mailbox.Extend("mailgun", inboundmailgun.Factory(a.Clock, replays))
	a.Mailbox.Extend("sendgrid", inboundsendgrid.Factory(a.Clock))
	a.Mailbox.Extend("resend", inboundresend.Factory(a.HTTP, a.Clock, replays))
	a.Mailbox.Extend("cloudflare", inboundcloudflare.Factory(a.Clock, replays))

	a.Notify.Extend("mail", mailchannel.Factory(a.Mail))
	a.Notify.Extend("database", notifydb.Factory(a.DB, a.Clock))
	a.Notify.Extend("broadcast", broadcast.Factory(a.Broadcast))
	a.Notify.Extend("webpush", webpush.Factory(a.HTTP, a.Clock, a.opts.WebPushGone, webpush.Defaults{
		Seed: a.DerivedKey("notifications.webpush.vapid"), Subject: a.Config.App.URL,
	}))

	a.Hash.Extend("argon2id", argon2id.Factory)
	a.Hash.Extend("bcrypt", bcrypt.Factory)

	a.Crypt.Extend("local", encryption.LocalFactory(a.appKey))
	a.Crypt.Extend("awskms", withAWS(awsConfig, awskms.Factory))
}

/*
registerWebDrivers registers the drivers of social sign-in, account
connections, custom domain certificates and language detection.
*/
func (a *Application) registerWebDrivers() {
	a.Social.Extend("github", github.Factory(a.HTTP))
	a.Social.Extend("google", google.Factory(a.HTTP, a.Clock))
	a.Social.Extend("oidc", oidc.Factory(a.HTTP, a.Clock))
	a.OAuthClient.Extend("generic", generic.Factory(a.HTTP))
	a.OAuthClient.Extend("slack", slack.Factory(a.HTTP, a.Clock))
	a.OAuthClient.Extend("hosted", hosted.Factory(a.HTTP))

	authorize := a.domainAuthorizer()
	a.Domains.Extend("none", tlsnone.Factory(authorize))
	a.Domains.Extend("caddyask", caddyask.Factory(authorize))
	a.Domains.Extend("certmagic", certmagic.Factory(a.DB, authorize, zapLogger(a.Log)))

	a.Languages.Extend("none", langnone.Factory)
	a.Languages.Extend("whatlang", whatlang.Factory)
}

/*
lazyAWS returns a function building the AWS configuration from the aws
section once, on first use, with every call sent through the guarded
HTTP client.
*/
func (a *Application) lazyAWS(ctx context.Context) func() (awssdk.Config, error) {
	return sync.OnceValues(func() (awssdk.Config, error) {
		return aws.Config(context.WithoutCancel(ctx), a.Config.AWS, a.HTTP)
	})
}

/*
withAWS adapts a driver factory that needs the AWS configuration into
one that builds it on first use.
*/
func withAWS[T any](
	awsConfig func() (awssdk.Config, error), factory func(awssdk.Config) manager.Factory[T],
) manager.Factory[T] {
	return func(name string, section config.Section) (T, error) {
		cfg, err := awsConfig()
		if err != nil {
			var zero T

			return zero, err
		}

		return factory(cfg)(name, section)
	}
}
