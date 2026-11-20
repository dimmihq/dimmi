/*
|--------------------------------------------------------------------------
| Send Failures
|--------------------------------------------------------------------------
|
| Meta refuses a send with a code and, for most causes, a subcode. One
| code means several things: 10 is a missing permission, a closed window
| or a thread another app answers, so the subcode is read first. Each
| refusal becomes one of the channel contract's errors with words a
| teammate can act on; only a refusal that may pass on its own, such as
| a rate limit or Meta being down, is tried again.
|
*/

package social

import (
	"errors"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
)

/*
Stable codes of failed sends, as teammates and the API see them.
*/
const (
	CodeWindowClosed     = "social_window_closed"
	CodeHumanOnly        = "social_human_only"
	CodeUnreachable      = "social_recipient_unreachable"
	CodeNotOwner         = "social_not_thread_owner"
	CodeAccessOff        = "social_message_access_off"
	CodeRestricted       = "social_account_restricted"
	CodeContentRejected  = "social_content_rejected"
	CodeConnectionBroken = "social_connection_broken"
	CodeThreadUnknown    = "social_thread_unknown"
	CodeRefused          = "social_refused"
	CodeUnavailable      = "social_unavailable"
)

/*
Subcodes Meta gives the refusals the inbox tells apart.
*/
const (
	subWindowInstagram = 2534022
	subWindowFacebook  = 2018278
	subWindowNews      = 2018065
	subNotOwnerPage    = 2018300
	subNotOwnerIG      = 2534037
	subAccessOff       = 2534041
	subCannotReceive   = 2018108
	subNotAvailable    = 1545041
	subNoUserPage      = 2018001
	subNoUserIG        = 2534014
	subRestrictedPage  = 1893063
	subRestrictedIG    = 2534029
	subFetchFailed     = 2018008
	subTypeMismatch    = 2018047
	subTooBig          = 2018109
	subVideoTimeout    = 2018294
)

const (
	windowClosedMessage = "More than 7 days passed since the customer's last message. " +
		"You can reply once they write again."
	humanOnlyMessage = "More than 24 hours passed since the customer's last message. " +
		"Only a teammate can reply now, not the AI agent."
	unreachableMessage = "This person is not receiving messages right now."
	restrictedMessage  = "Meta has restricted this account from sending messages."
	notOwnerMessage    = "Another app is answering this conversation. Take it over in Meta Business Suite first."
	accessOffMessage   = `Turn on "Allow access to messages" in the Instagram app's message settings, ` +
		"then send again."
)

/*
MapError turns a failed Graph call into the channel contract's error.
An error that is not Meta's refusal, such as a timeout, is transient.
*/
func MapError(err error) error {
	refused, ok := errors.AsType[*meta.GraphError](err)
	if !ok {
		return err
	}

	if wait, limited := pace(refused); limited {
		return &contracts.RateLimitedError{RetryAfter: wait}
	}

	named := classify(refused)

	return &contracts.SendError{Code: named.code, Message: named.message, Err: named.sentinel}
}

/*
pace returns how long to wait when Meta asks the app to slow down.
*/
func pace(e *meta.GraphError) (time.Duration, bool) {
	switch e.Code {
	case 4, 17, 341, 80001:
		return time.Minute, true
	case 613:
		return time.Minute, e.Subcode != 2018338
	default:
		return 0, false
	}
}

/*
refusal names what Meta refused: its stable code, the words teammates
read and the contract's sentinel, nil for one tried again.
*/
type refusal struct {
	code     string
	message  string
	sentinel error
}

/*
classify names a refusal by its subcode when that pins it down, else by
its code.
*/
func classify(e *meta.GraphError) refusal {
	if named, ok := bySubcode(e.Subcode); ok {
		return named
	}

	switch e.Code {
	case 190, 102, 10, 200, 3:
		return refusal{
			CodeConnectionBroken, "Meta no longer accepts this connection. Reconnect it.",
			contracts.ErrConnectionBroken,
		}
	case 551:
		return refusal{CodeUnreachable, unreachableMessage, contracts.ErrRecipientUnreachable}
	case 368, 10904, 2022:
		return refusal{CodeRestricted, restrictedMessage, contracts.ErrPermanent}
	case 100, 613, 10903, 36103:
		return refusal{CodeRefused, "Meta refused the message.", contracts.ErrPermanent}
	default:
		return refusal{CodeUnavailable, "Meta is not answering. The message will be tried again.", nil}
	}
}

/*
bySubcode names the refusals a subcode pins down.
*/
func bySubcode(sub int) (refusal, bool) {
	switch sub {
	case subWindowInstagram, subWindowFacebook, subWindowNews:
		return refusal{CodeWindowClosed, windowClosedMessage, contracts.ErrWindowClosed}, true
	case subNotOwnerPage, subNotOwnerIG:
		return refusal{CodeNotOwner, notOwnerMessage, contracts.ErrPermanent}, true
	case subAccessOff:
		return refusal{CodeAccessOff, accessOffMessage, contracts.ErrConnectionBroken}, true
	case subCannotReceive, subNotAvailable, subNoUserPage, subNoUserIG:
		return refusal{CodeUnreachable, unreachableMessage, contracts.ErrRecipientUnreachable}, true
	case subRestrictedPage, subRestrictedIG:
		return refusal{CodeRestricted, restrictedMessage, contracts.ErrPermanent}, true
	case subFetchFailed, subTypeMismatch, subTooBig, subVideoTimeout:
		return refusal{CodeContentRejected, "Meta could not take this file.", contracts.ErrContentRejected}, true
	default:
		return refusal{}, false
	}
}
