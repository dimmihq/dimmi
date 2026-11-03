/*
|--------------------------------------------------------------------------
| Mail In Tests
|--------------------------------------------------------------------------
|
| Mail leaves the process, so tests swap its transport for the fake and
| run the real queue that sends it. A test then reads the mail a user
| would get, follows its link or types its code, exactly as they would.
|
*/

package tests

import (
	"net/url"
	"slices"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
Mail swaps the application's mail transport for a fake, for the rest of
the test, and returns it. Every call returns the same fake.
*/
func (tc *TestCase) Mail() *mail.Fake {
	tc.mailOnce.Do(func() { tc.mail = tc.App.Mail.Fake() })

	return tc.mail
}

/*
Drain runs every queued job that is ready, mail included.
*/
func (tc *TestCase) Drain() {
	tc.T.Helper()

	require.NoError(tc.T, tc.App.Queue.Drain(tc.T.Context()))
}

/*
Sent drains the queue and returns every mailable of type M the fake
recorded, oldest first, with the message each one became.
*/
func Sent[M mail.Mailable](tc *TestCase, fake *mail.Fake) ([]M, []mail.SentMessage) {
	tc.T.Helper()
	tc.Drain()

	var (
		mailables []M
		messages  []mail.SentMessage
	)

	for _, sent := range fake.Sent() {
		if m, ok := sent.Mailable.(M); ok {
			mailables, messages = append(mailables, m), append(messages, sent)
		}
	}

	return mailables, messages
}

/*
LastSent drains the queue and returns the newest mailable of type M,
with the message it became, failing the test when none was sent.
*/
func LastSent[M mail.Mailable](tc *TestCase, fake *mail.Fake) (M, mail.SentMessage) {
	tc.T.Helper()

	mailables, messages := Sent[M](tc, fake)
	require.NotEmpty(tc.T, mailables, "no %T was sent", *new(M))

	return mailables[len(mailables)-1], messages[len(messages)-1]
}

/*
Fragment returns the values a link carries after its #, where account
mail puts its secrets.
*/
func (tc *TestCase) Fragment(link string) url.Values {
	tc.T.Helper()

	u, err := url.Parse(link)
	require.NoError(tc.T, err)

	values, err := url.ParseQuery(u.Fragment)
	require.NoError(tc.T, err)

	return values
}

/*
SentTo returns the addresses a message went to.
*/
func SentTo(sent mail.SentMessage) []string {
	var to []string
	for _, address := range sent.Message.To {
		to = append(to, address.Email)
	}

	return slices.Clip(to)
}
