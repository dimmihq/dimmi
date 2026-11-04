/*
|--------------------------------------------------------------------------
| Channel Send Results
|--------------------------------------------------------------------------
|
| What a channel reports after trying to deliver a message. Success
| carries the provider's id of the message. Failures carry their meaning
| in sentinels: a permanent failure ends a message, an unwrapped error
| is transient and retried, and a rate limit postpones without spending
| an attempt. Nothing reported ever holds message content or secrets.
|
*/

package contracts

import (
	"errors"
	"time"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
SendResult is what became of a delivered message.

ChannelMessageID is the provider's id of the message. ExternalThreadKey
is set when the send opened the thread. SentAt is the provider's time,
zero when it gave none. Metadata is kept on the message under the
channel's type.
*/
type SendResult struct {
	ChannelMessageID  string
	Status            enums.DeliveryStatus
	ExternalThreadKey string
	SentAt            time.Time
	Metadata          map[string]any
}

/*
Errors a channel wraps. An error wrapping none of them is transient.
*/
var (
	ErrPermanent            = errors.New("channel: permanent failure")
	ErrRecipientUnreachable = errors.New("channel: recipient unreachable")
	ErrWindowClosed         = errors.New("channel: messaging window closed")
	ErrConnectionBroken     = errors.New("channel: connection credentials invalid or revoked")
	ErrContentRejected      = errors.New("channel: content rejected by provider")
	ErrUnsupported          = errors.New("channel: operation unsupported")
)

/*
RateLimitedError postpones a send by RetryAfter without spending an
attempt.
*/
type RateLimitedError struct {
	RetryAfter time.Duration
}

/*
Error describes the limit.
*/
func (e *RateLimitedError) Error() string {
	return "channel: rate limited"
}

/*
SendError carries a stable code and a message members can read. Err is
one of the sentinels, or nil for a transient failure. Neither ever holds
message content or secrets.
*/
type SendError struct {
	Code    string
	Message string
	Err     error
}

/*
Error returns the code and the message.
*/
func (e *SendError) Error() string {
	return e.Code + ": " + e.Message
}

/*
Unwrap returns the sentinel the error carries.
*/
func (e *SendError) Unwrap() error {
	return e.Err
}
