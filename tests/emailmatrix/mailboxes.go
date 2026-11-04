//go:build emailmatrix

/*
|--------------------------------------------------------------------------
| Mailboxes
|--------------------------------------------------------------------------
|
| Six real hosts decide whether mail reaches people. The harness speaks to
| each through two small interfaces, one to send and one to read, so a host
| changes how it is reached without any scenario changing. A way of reaching a
| host that does not exist yet reports the scenario as blocked, never as passed.
|
*/

package emailmatrix

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/smtp"
)

/*
errBlocked marks a step that could not be attempted, as opposed to one that
was attempted and failed.
*/
var errBlocked = errors.New("blocked")

/*
mailboxHost describes one host and how it is reached.
*/
type mailboxHost struct {
	Name           string
	ReadsWith      string
	ReadsWithToken bool
	SendsWithToken bool
}

func mailboxHosts() []mailboxHost {
	return []mailboxHost{
		{Name: "gmail", ReadsWith: "imap"},
		{Name: "workspace", ReadsWith: "gmail api", ReadsWithToken: true},
		{Name: "outlook", ReadsWith: "graph", ReadsWithToken: true},
		{Name: "m365", ReadsWith: "graph", ReadsWithToken: true, SendsWithToken: true},
		{Name: "yahoo", ReadsWith: "imap"},
		{Name: "icloud", ReadsWith: "imap"},
	}
}

/*
sender sends mail from a mailbox the way its owner's client would.
*/
type sender interface {
	Send(ctx context.Context, msg mail.Message) error
}

/*
reader finds a message holding the marker in a mailbox.
*/
type reader interface {
	Find(ctx context.Context, marker string) (received, error)
}

/*
received is a message as the mailbox host shows it. Found is false while it
has not arrived. Folder is inbox or spam, Headers the header block as delivered.
*/
type received struct {
	Found   bool
	Folder  string
	Headers map[string][]string
	HTML    string
	Text    string
}

/*
mailbox is one host's address with its way in and out.
*/
type mailbox struct {
	Host    string
	Address string
	Out     sender
	In      reader
}

/*
smtpSender sends over the host's submission port with the framework's mailer.
*/
type smtpSender struct {
	mailer *smtp.Mailer
}

/*
Send hands the message to the submission server.
*/
func (s smtpSender) Send(ctx context.Context, msg mail.Message) error {
	if _, err := s.mailer.Send(ctx, msg); err != nil {
		return fmt.Errorf("submit mail: %w", err)
	}

	return nil
}

/*
unavailableSender and unavailableReader stand in for a way of reaching a host
that does not exist yet, and block every step that needs it.
*/
type unavailableSender struct {
	via string
}

/*
Send refuses, since this way of sending is not built.
*/
func (s unavailableSender) Send(context.Context, mail.Message) error {
	return fmt.Errorf("%w: sending through %s is not built", errBlocked, s.via)
}

type unavailableReader struct {
	via string
}

/*
Find refuses, since this way of reading is not built.
*/
func (r unavailableReader) Find(context.Context, string) (received, error) {
	return received{}, fmt.Errorf("%w: reading through %s is not built", errBlocked, r.via)
}

/*
openMailboxes builds the six mailboxes from their credentials.
*/
func openMailboxes(clk clock.Clock, secrets map[string]mailboxSecret) ([]mailbox, error) {
	boxes := make([]mailbox, 0, len(secrets))

	for _, host := range mailboxHosts() {
		secret := secrets[host.Name]

		out, err := openSender(clk, host, secret)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", host.Name, err)
		}

		boxes = append(boxes, mailbox{
			Host:    host.Name,
			Address: secret.Address,
			Out:     out,
			In:      unavailableReader{via: host.ReadsWith},
		})
	}

	return boxes, nil
}

func openSender(clk clock.Clock, host mailboxHost, secret mailboxSecret) (sender, error) {
	if host.SendsWithToken {
		return unavailableSender{via: host.ReadsWith}, nil
	}

	mailer, err := smtp.New(clk, smtp.Options{
		Host:     secret.SMTPHost,
		Port:     secret.SMTPPort,
		Username: secret.SMTPUsername,
		Password: secret.SMTPPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("smtp mailer: %w", err)
	}

	return smtpSender{mailer: mailer}, nil
}
