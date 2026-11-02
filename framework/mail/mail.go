/*
|--------------------------------------------------------------------------
| Mail Messages
|--------------------------------------------------------------------------
|
| A message is plain data every driver understands: addresses, subject,
| a text and an HTML body, headers and attachments, or a message already
| built as MIME. Drivers report failures as one of three kinds, so the
| caller decides on retries without knowing the provider: a permanent
| failure is never retried, a temporary one is, and a rate limit says
| when to try again.
|
*/

package mail

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"time"
)

/*
Errors a mailer returns, matched with errors.Is.

ErrPermanent means the message can never be sent as it is: an invalid
address, a rejected sender or content. ErrTemporary means a later try
may succeed: a timeout, a provider outage, a refused login.
ErrInvalidAddress rejects text that is not an email address.
*/
var (
	ErrPermanent      = errors.New("mail: permanent failure")
	ErrTemporary      = errors.New("mail: temporary failure")
	ErrInvalidAddress = errors.New("mail: invalid address")
)

/*
RateLimitedError means the provider asked to slow down. RetryAfter is
how long it asked to wait, zero when it did not say.
*/
type RateLimitedError struct {
	RetryAfter time.Duration
}

/*
Error describes the limit.
*/
func (e *RateLimitedError) Error() string {
	if e.RetryAfter == 0 {
		return "mail: rate limited"
	}

	return fmt.Sprintf("mail: rate limited, retry after %s", e.RetryAfter)
}

/*
Address is one mailbox: an email address with an optional display name.
*/
type Address struct {
	Name  string `json:"name,omitempty" koanf:"name"`
	Email string `json:"email"          koanf:"email"`
}

/*
String formats the address for a header, quoting and encoding the name
when it needs it.
*/
func (a Address) String() string {
	return (&mail.Address{Name: a.Name, Address: a.Email}).String()
}

/*
ParseAddress reads one address such as "Ana <ana@example.com>".
*/
func ParseAddress(s string) (Address, error) {
	parsed, err := mail.ParseAddress(s)
	if err != nil {
		return Address{}, fmt.Errorf("%w: %q", ErrInvalidAddress, s)
	}

	return Address{Name: parsed.Name, Email: parsed.Address}, nil
}

/*
Attachment is a file sent with a message. An inline attachment is shown
in the HTML body, which points at it with "cid:" and its ContentID.
*/
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
	Inline      bool
	ContentID   string
}

/*
Message is one mail to send.

Headers adds header fields such as Message-ID, In-Reply-To, References
or List-Unsubscribe; the address, subject and content fields cannot be
set through it. Raw is a message already built as MIME: when set, the
body fields are ignored and the addresses, when empty, are read from its
headers. Bcc recipients are never written into the message. Tags label
the message at the provider for matching its delivery events later.
ConfigurationSet and TenantName are used by SES only.
*/
type Message struct {
	From, ReplyTo    Address
	To, Cc, Bcc      []Address
	Subject          string
	Text, HTML       string
	Headers          map[string]string
	Attachments      []Attachment
	Raw              []byte
	Tags             map[string]string
	ConfigurationSet string
	TenantName       string
}

/*
SendResult is what the provider reported for an accepted message.
ProviderMessageID is the provider's own id, used to match its delivery
events. MessageID is the Message-ID header actually sent, without angle
brackets, when the provider replaced ours; empty means ours was kept or
the provider does not say.
*/
type SendResult struct {
	ProviderMessageID string
	MessageID         string
}

/*
Mailer sends messages. Implementations are safe for concurrent use.
*/
type Mailer interface {
	/*
		Send delivers the message to the provider, which accepts it for
		delivery. Errors match ErrPermanent, ErrTemporary or
		*RateLimitedError.
	*/
	Send(ctx context.Context, msg Message) (SendResult, error)
}
