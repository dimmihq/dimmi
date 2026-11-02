/*
|--------------------------------------------------------------------------
| Fake Mailer
|--------------------------------------------------------------------------
|
| Tests assert that mail was sent without sending any. The fake stands
| in for every mailer at once, checks each message exactly as the real
| drivers do, and records it with the mailer name it was sent on and the
| mailable it came from, if any.
|
|   mails := a.Mail.Fake()
|   ... run the action ...
|   mail.AssertSent(t, mails, func(m SignInMail) bool { return m.Email == email })
|
*/

package mail

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
SentMessage is a message the fake recorded. Mailable is nil for a
message sent directly on a mailer. Locale is the locale the message was
rendered in, "" when none was set.
*/
type SentMessage struct {
	Mailer   string
	Message  Message
	Mailable Mailable
	Locale   string
}

/*
Fake records messages instead of sending them. It is safe for concurrent
use.
*/
type Fake struct {
	mu   sync.Mutex
	sent []SentMessage
}

/*
Sent returns every recorded message, oldest first.
*/
func (f *Fake) Sent() []SentMessage {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.sent)
}

/*
AssertMessage fails the test unless some recorded message matches.
*/
func (f *Fake) AssertMessage(tb testing.TB, match func(sent SentMessage) bool) {
	tb.Helper()

	if slices.ContainsFunc(f.Sent(), match) {
		return
	}

	tb.Errorf("mail: no matching message among %d sent", len(f.Sent()))
}

/*
AssertNothingSent fails the test when any message was recorded.
*/
func (f *Fake) AssertNothingSent(tb testing.TB) {
	tb.Helper()

	if sent := f.Sent(); len(sent) > 0 {
		tb.Errorf("mail: expected no messages, %d sent", len(sent))
	}
}

/*
AssertSent fails the test unless a mailable of type M matching the
function was sent.
*/
func AssertSent[M Mailable](tb testing.TB, f *Fake, match func(mailable M) bool) {
	tb.Helper()

	for _, sent := range f.Sent() {
		if mailable, ok := sent.Mailable.(M); ok && match(mailable) {
			return
		}
	}

	var zero M
	tb.Errorf("mail: no matching %T sent", zero)
}

func (f *Fake) mailer(name string) Mailer {
	return fakeMailer{fake: f, name: name}
}

/*
record checks the message as a driver would and records it. The
provider id counts the messages, so every one gets its own.
*/
func (f *Fake) record(ctx context.Context, name string, msg Message, mailable Mailable) (SendResult, error) {
	if err := ctx.Err(); err != nil {
		return SendResult{}, fmt.Errorf("fake send: %w: %w", ErrTemporary, err)
	}

	if err := msg.Validate(); err != nil {
		return SendResult{}, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, SentMessage{Mailer: name, Message: msg, Mailable: mailable, Locale: i18n.Locale(ctx)})

	return SendResult{ProviderMessageID: fmt.Sprintf("fake-%d", len(f.sent))}, nil
}

/*
fakeMailer is the fake standing in for one named mailer.
*/
type fakeMailer struct {
	fake *Fake
	name string
}

/*
Send records the message under the mailer's name.
*/
func (m fakeMailer) Send(ctx context.Context, msg Message) (SendResult, error) {
	return m.fake.record(ctx, m.name, msg, nil)
}
