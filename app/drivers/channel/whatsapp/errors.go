/*
|--------------------------------------------------------------------------
| Meta's Errors
|--------------------------------------------------------------------------
|
| Meta answers a refused send with a numeric code. Each is mapped onto
| the channel contract once, here: a closed window, a customer WhatsApp
| cannot reach, a template Meta will not send, a token or number that no
| longer works, or a pace to slow down to. Anything unknown, and Meta's
| own trouble, is tried again. Teammates read the reason in plain words,
| never Meta's internals.
|
*/

package whatsapp

import (
	"errors"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Stable codes of failed sends, as teammates and the API see them.
*/
const (
	CodeWindowClosed     = "whatsapp_window_closed"
	CodeUnreachable      = "whatsapp_recipient_unreachable"
	CodePayment          = "whatsapp_payment_required"
	CodeLimited          = "whatsapp_limited"
	CodeTemplateRejected = "whatsapp_template_rejected"
	CodeContentRejected  = "whatsapp_content_rejected"
	CodeConnectionBroken = "whatsapp_connection_broken"
	CodeThreadUnknown    = "whatsapp_thread_unknown"
	CodeRefused          = "whatsapp_refused"
	CodeUnavailable      = "whatsapp_unavailable"
)

/*
windowClosedMessage tells teammates why free text did not go out.
*/
const windowClosedMessage = "More than 24 hours passed since the customer's last message. " +
	"Send an approved template."

/*
MapError turns a failed Graph call into the channel contract's error.
An error that is not Meta's refusal, such as a timeout, is transient.
*/
func MapError(err error) error {
	refused, ok := errors.AsType[*GraphError](err)
	if !ok {
		return err
	}

	if wait, limited := pace(refused.Code); limited {
		return &contracts.RateLimitedError{RetryAfter: wait}
	}

	code, message, sentinel := classify(refused)

	return &contracts.SendError{Code: code, Message: message, Err: sentinel}
}

/*
pace returns how long to wait before trying again when Meta asks the
business to slow down.
*/
func pace(code int) (time.Duration, bool) {
	switch code {
	case 131056:
		return 6 * time.Second, true
	case 130429, 4:
		return time.Second, true
	case 80007, 80008:
		return time.Minute, true
	default:
		return 0, false
	}
}

/*
classify names a refusal: its stable code, the words teammates read and
the contract's sentinel, nil for one tried again.
*/
func classify(e *GraphError) (string, string, error) {
	switch {
	case e.Code == 131047:
		return CodeWindowClosed, windowClosedMessage, contracts.ErrWindowClosed
	case e.Code == 131026:
		return CodeUnreachable, "WhatsApp could not deliver to this number.", contracts.ErrRecipientUnreachable
	case e.Code == 131042:
		return CodePayment, "Meta refused the message over a payment issue on the WhatsApp account.",
			contracts.ErrPermanent
	case e.Code == 131048 || e.Code == 131049 || e.Code == 131050:
		return CodeLimited, "Meta limited messages to this customer for now.", contracts.ErrPermanent
	case e.Code >= 132000 && e.Code < 133000:
		return CodeTemplateRejected, "Meta refused the template. Check it is approved and its variables match.",
			contracts.ErrContentRejected
	case e.Code == 131051 || e.Code == 131052 || e.Code == 131053 || e.Code == 131009:
		return CodeContentRejected, "WhatsApp does not accept this message or file.", contracts.ErrContentRejected
	case e.Code == 190 || e.Code == 10 || e.Code == 200 || e.Code == 131031 || e.Code == 133010 ||
		e.Code == 368 || e.Code == 131005:
		return CodeConnectionBroken, "The WhatsApp connection must be reconnected.", contracts.ErrConnectionBroken
	case e.Code == 100:
		return CodeRefused, "Meta refused the message.", contracts.ErrPermanent
	default:
		return CodeUnavailable, "WhatsApp is not answering. The message will be tried again.", nil
	}
}
