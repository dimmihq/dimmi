/*
|--------------------------------------------------------------------------
| Send Failures
|--------------------------------------------------------------------------
|
| What the mailer reported, translated for the inbox. A provider asking
| to slow down postpones the reply without spending an attempt. A
| refusal fails the reply at once: an unknown or invalid recipient as
| unreachable, anything else as permanent. Every other failure passes,
| such as a timeout or a provider briefly away, and the inbox retries
| it. A provider's own credentials failing is the install's fault, not
| the workspace's, so it is retried too while an operator fixes it.
|
*/

package email

import (
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
sendError maps a mailer error to the channel's errors.
*/
func sendError(err error) error {
	if limited, ok := errors.AsType[*mail.RateLimitedError](err); ok {
		return &contracts.RateLimitedError{RetryAfter: limited.RetryAfter}
	}

	message := diagnostic(err.Error())

	switch {
	case errors.Is(err, mail.ErrInvalidAddress) || (errors.Is(err, mail.ErrPermanent) && recipientFault(message)):
		return &contracts.SendError{
			Code: "recipient_rejected", Message: message, Err: contracts.ErrRecipientUnreachable,
		}
	case errors.Is(err, mail.ErrPermanent):
		return &contracts.SendError{Code: "provider_rejected", Message: message, Err: contracts.ErrPermanent}
	}

	return &contracts.SendError{Code: "provider_unavailable", Message: message}
}

/*
recipientFault reports whether a refusal blames the recipient: an
enhanced status code of the 5.1 family, such as 5.1.1 user unknown.
*/
func recipientFault(message string) bool {
	return strings.Contains(message, "5.1.") || strings.Contains(strings.ToLower(message), "recipient")
}

/*
permanent reports whether a mapped failure ends the reply.
*/
func permanent(err error) bool {
	return errors.Is(err, contracts.ErrPermanent) || errors.Is(err, contracts.ErrRecipientUnreachable)
}
