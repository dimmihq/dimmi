/*
|--------------------------------------------------------------------------
| SES Events Controller
|--------------------------------------------------------------------------
|
| POST /webhooks/ses/events is where Amazon SNS hands over what SES
| says became of sent mail. The post is read only once SNS's signature
| checks out for a topic the install named; a bounce or a complaint is
| then recorded on the email it is about, and everything else is
| answered and dropped.
|
| SNS retries anything but success, so a post that cannot be trusted is
| refused for good and a failure to record asks for another try.
|
*/

package webhooks

import (
	"errors"
	"io"

	"github.com/dimmihq/dimmi/framework/aws/sesevents"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"

	"github.com/dimmihq/dimmi/app/actions/email"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
sesTransport is the transport sent emails record for SES.
*/
const sesTransport = "ses"

/*
SESEventsController receives SES delivery events. Enabled is false on
an install that named no events topic, which answers not found.
*/
type SESEventsController struct {
	Events  *sesevents.Reader
	Record  *email.RecordDeliveryEvent
	Enabled bool
	Log     log.Logger
	Errors  fhttp.Errors
}

/*
Store reads one SNS post and records the bounce or complaint in it.
*/
func (c *SESEventsController) Store(w fhttp.ResponseWriter, r *fhttp.Request) {
	if !c.Enabled {
		c.Errors.Render(w, r, exceptions.ErrInboundDriverDisabled)

		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, sesevents.MaxBytes+1))
	if err != nil || len(body) > sesevents.MaxBytes {
		w.WriteHeader(fhttp.StatusRequestEntityTooLarge)

		return
	}

	event, err := c.Events.Receive(r.Context(), body)
	if err == nil && event != nil {
		err = c.Record.Handle(r.Context(), deliveryEvent(*event))
	}

	switch {
	case err == nil:
		w.WriteHeader(fhttp.StatusOK)
	case errors.Is(err, sesevents.ErrRejected):
		w.WriteHeader(fhttp.StatusUnauthorized)
	default:
		c.Log.ErrorContext(r.Context(), "record ses event", log.Err(err))
		w.WriteHeader(fhttp.StatusServiceUnavailable)
	}
}

/*
deliveryEvent turns an SES event into the report the email channel
records. A complaint that says the mail was wanted names nobody, so
nothing is recorded for it.
*/
func deliveryEvent(e sesevents.Event) email.DeliveryEvent {
	out := email.DeliveryEvent{
		Transport: sesTransport, ProviderMessageID: e.MessageID, Complaint: e.Kind == sesevents.KindComplaint,
		Permanent: e.Permanent,
	}

	if e.NotSpam {
		return out
	}

	for _, r := range e.Recipients {
		out.Recipients = append(out.Recipients, email.DeliveryEventRecipient{
			Address: r.Address, Status: r.Status, Diagnostic: r.Diagnostic,
		})
	}

	return out
}
