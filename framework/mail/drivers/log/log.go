/*
|--------------------------------------------------------------------------
| Log Mailer
|--------------------------------------------------------------------------
|
| Before a provider is configured, and in local development, mail is
| written to the log instead of being sent: the whole MIME document, so
| a developer can read every header and follow every link in it. That
| includes sign-in links, which is why this driver must never carry real
| users' mail in production.
|
*/

package log

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Mailer writes messages to a logger.
*/
type Mailer struct {
	logger  log.Logger
	builder *mime.Builder
	name    string
}

/*
Factory builds log mailers for the mail manager.
*/
func Factory(logger log.Logger, clk clock.Clock) manager.Factory[mail.Mailer] {
	return func(name string, _ config.Section) (mail.Mailer, error) {
		return New(logger, clk, name), nil
	}
}

/*
New returns a mailer writing to the logger under the mailer name,
dating messages by the clock.
*/
func New(logger log.Logger, clk clock.Clock, name string) *Mailer {
	return &Mailer{logger: logger, builder: mime.NewBuilder(clk), name: name}
}

/*
Send writes the message as one info line.
*/
func (m *Mailer) Send(ctx context.Context, msg mail.Message) (mail.SendResult, error) {
	if err := ctx.Err(); err != nil {
		return mail.SendResult{}, fmt.Errorf("log send: %w: %w", mail.ErrTemporary, err)
	}

	recipients, err := msg.Recipients()
	if err != nil {
		return mail.SendResult{}, err
	}

	raw, err := m.builder.Build(msg)
	if err != nil {
		return mail.SendResult{}, err
	}

	m.logger.InfoContext(ctx, "mail sent",
		log.String("mailer", m.name),
		log.String("recipients", strings.Join(provider.Emails(recipients), ", ")),
		log.String("message", string(raw)),
	)

	return mail.SendResult{}, nil
}
