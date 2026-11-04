/*
|--------------------------------------------------------------------------
| Email Channel
|--------------------------------------------------------------------------
|
| The email channel as the inbox sees it. Replies are pushed by a job
| and may start a conversation; they carry a subject, attachments and
| copies, and are retried for a day, the way mail servers retry. Only
| providers that report deliveries make delivery receipts possible; a
| plain relay only says it took the message.
|
| Sending itself is the email actions' work, behind Sender, so the
| channel stays free of storage.
|
*/

package email

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
retryWindow is how long a reply that failed for a passing reason keeps
being retried.
*/
const retryWindow = 24 * time.Hour

/*
Sender sends one reply by email.
*/
type Sender interface {
	/*
		Send delivers the message on the connection and reports what the
		provider said.
	*/
	Send(
		ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage,
	) (contracts.SendResult, error)
}

/*
Channel is the email channel driver, registered with the inbox's
channel manager so replies on email conversations go out through it.
Transport is the driver of the
customer stream's mailer, which decides whether deliveries are
reported.
*/
type Channel struct {
	Sender    Sender
	Transport string
}

/*
Type names the email connection type.
*/
func (c *Channel) Type() string {
	return string(enums.ChannelTypeEmail)
}

/*
Capabilities returns what email can do, the same on every address.
*/
func (c *Channel) Capabilities(models.ChannelConnection) contracts.ChannelCapabilities {
	return contracts.ChannelCapabilities{
		Transport: contracts.TransportPush, StartConversation: contracts.StartAlways,
		Formatting: contracts.FormatHTML, Subject: true, Attachments: true, ParticipantsCC: true,
		DeliveryReceipts: c.reportsDeliveries(), RetryWindow: retryWindow,
	}
}

/*
Send delivers one reply.
*/
func (c *Channel) Send(
	ctx context.Context, conn models.ChannelConnection, msg contracts.OutboundMessage,
) (contracts.SendResult, error) {
	return c.Sender.Send(ctx, conn, msg)
}

/*
reportsDeliveries reports whether the transport tells when mail was
delivered.
*/
func (c *Channel) reportsDeliveries() bool {
	switch c.Transport {
	case "ses", "postmark", "resend", "mailgun", "sendgrid":
		return true
	}

	return false
}
