/*
|--------------------------------------------------------------------------
| Mailbox Controller
|--------------------------------------------------------------------------
|
| POST /webhooks/{provider}/... is where each receiving provider hands
| over mail. One controller serves them all: the source checks the
| provider's own proof and reads the delivery, then the mail joins the
| same pipeline as any other.
|
| The provider must never lose mail, so nothing is answered with success
| until the raw message is kept. Whatever else goes wrong asks the
| provider to retry, and each source decides which answer its provider
| reads as "stop trying". A delivery counts as seen only after the raw
| message is kept, so a failed attempt can always come back.
|
*/

package webhooks

import (
	"context"
	"errors"
	"io"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/telemetry"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
How long a webhook waits for memory to read a message into, and for the
raw message to be kept. Providers retry, so giving up early is safe.
*/
const (
	budgetWait  = 2 * time.Second
	keepRawWait = 20 * time.Second
)

/*
MailboxController receives mail from every webhook-fed source.
*/
type MailboxController struct {
	Sources *mailbox.Manager
	Receive *email.ReceiveInboundEmail
	Health  *email.InboundHealth
	Budget  *mailbox.Budget
	Metrics *email.InboundMetrics
	Config  appconfig.EmailConfig
	Log     log.Logger
	Errors  fhttp.Errors
}

/*
Store returns the handler of the named source.
*/
func (c *MailboxController) Store(source string) fhttp.HandlerFunc {
	return func(w fhttp.ResponseWriter, r *fhttp.Request) {
		receiver, err := c.Sources.Receiver(source)
		if err != nil || !c.Config.SourceEnabled(source) {
			c.Errors.Render(w, r, exceptions.ErrInboundDriverDisabled)

			return
		}

		w.WriteHeader(mailbox.Status(receiver, c.deliver(r, receiver)))
	}
}

/*
deliver reads the delivery under the memory budget and keeps the mail.
*/
func (c *MailboxController) deliver(r *fhttp.Request, receiver mailbox.Receiver) error {
	source := telemetry.String("source", receiver.Name())
	ctx, span := c.Metrics.Telemetry.Start(r.Context(), "email.inbound.receive", source)
	defer span.End()

	r = r.WithContext(ctx)

	release, err := c.Budget.Acquire(r.Context(), declaredSize(r, receiver), budgetWait)
	if err != nil {
		return err
	}
	defer release()

	env, err := receiver.Receive(r.Context(), mailbox.Delivery{
		Header: r.Header, Query: r.URL.Query(), Size: r.ContentLength,
		Body: io.LimitReader(r.Body, receiver.MaxWebhookBytes()+1),
	})
	c.observe(r.Context(), receiver.Name(), env, err)

	if err != nil || env == nil {
		return err
	}

	return c.keep(r.Context(), *env)
}

/*
declaredSize is what a delivery will hold in memory: its declared length,
or unknown for a source that fetches the raw message itself.
*/
func declaredSize(r *fhttp.Request, receiver mailbox.Receiver) int64 {
	if fetcher, ok := receiver.(mailbox.RawFetcher); ok && fetcher.FetchesRaw() {
		return -1
	}

	return r.ContentLength
}

/*
keep stores the raw message and queues its processing, then lets the
source remember the delivery. Failing to remember it loses nothing: the
pipeline recognizes a message it already holds.
*/
func (c *MailboxController) keep(ctx context.Context, env mailbox.Envelope) error {
	ctx, cancel := context.WithTimeout(ctx, keepRawWait)
	defer cancel()

	received, err := c.Receive.Handle(ctx, env)
	if err != nil {
		return err
	}

	c.Metrics.Received(ctx, env.Source, env.SizeBytes, len(received))

	if env.Accepted == nil {
		return nil
	}

	if err := env.Accepted(ctx); err != nil {
		c.Log.WarnContext(ctx, "remember inbound delivery", log.String("source", env.Source), log.Err(err))
	}

	return nil
}

/*
observe leaves what an operator needs to know about a delivery: a setting
at the provider that makes deliveries useless or risky, or the provider
having ended the subscription.
*/
func (c *MailboxController) observe(ctx context.Context, source string, env *mailbox.Envelope, err error) {
	var misconfigured *mailbox.MisconfiguredError

	switch {
	case errors.As(err, &misconfigured):
		c.Health.RecordMisconfigured(ctx, source, misconfigured.Setting)
	case errors.Is(err, mailbox.ErrUnsubscribed):
		c.Health.RecordMisconfigured(ctx, source, "subscription ended")
		c.Log.WarnContext(ctx, "inbound subscription ended", log.String("source", source))
	case env != nil && env.Advice != "":
		c.Health.RecordMisconfigured(ctx, source, env.Advice)
	case err == nil:
		c.Health.ClearMisconfigured(ctx, source)
	}
}
