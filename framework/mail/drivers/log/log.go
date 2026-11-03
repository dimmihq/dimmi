/*
|--------------------------------------------------------------------------
| Log Mailer
|--------------------------------------------------------------------------
|
| Before a provider is configured, and in local development, mail is
| written to the log instead of being sent. Every line is marked
| mail_fallback=true, so an operator searching the log finds each mail
| that never left, with its subject and recipients.
|
| How much of the mail a line shows is the operator's choice, because a
| mail may carry sign-in links:
|
|   content: message   the whole MIME document and every link (default)
|   content: links     subject, recipients and every link, no body
|   content: envelope  subject and recipients only, nothing secret
|   level: warn        write at WARN, so the line stands out (default info)
|
| A server whose mail carries real users' secrets must never write the
| whole message or its links, which is what envelope is for.
|
*/

package log

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"slices"
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
The amounts of a mail a line can show.
*/
const (
	ContentMessage  = "message"
	ContentLinks    = "links"
	ContentEnvelope = "envelope"
)

/*
ErrInvalidOption is returned by the factory for a level or content it
does not know.
*/
var ErrInvalidOption = errors.New("mail log: invalid option")

/*
linkPattern finds the web addresses in a mail body.
*/
var linkPattern = regexp.MustCompile(`https?://[^\s<>"'()]+`)

/*
Options are the mailer's settings: the level lines are written at and
how much of each mail they show.
*/
type Options struct {
	Level   string `default:"info"    koanf:"level"`
	Content string `default:"message" koanf:"content"`
}

/*
Mailer writes messages to a logger.
*/
type Mailer struct {
	logger  log.Logger
	builder *mime.Builder
	name    string
	level   slog.Level
	content string
}

/*
Factory builds log mailers for the mail manager from their options.
*/
func Factory(logger log.Logger, clk clock.Clock) manager.Factory[mail.Mailer] {
	return func(name string, section config.Section) (mail.Mailer, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, fmt.Errorf("mail log %s: %w", name, err)
		}

		return NewWith(logger, clk, name, opts)
	}
}

/*
New returns a mailer writing whole messages at INFO to the logger under
the mailer name, dating messages by the clock.
*/
func New(logger log.Logger, clk clock.Clock, name string) *Mailer {
	return &Mailer{logger: logger, builder: mime.NewBuilder(clk), name: name, content: ContentMessage}
}

/*
NewWith returns a mailer with the options, or ErrInvalidOption for a
level or content it does not know.
*/
func NewWith(logger log.Logger, clk clock.Clock, name string, opts Options) (*Mailer, error) {
	m := New(logger, clk, name)

	if err := m.level.UnmarshalText([]byte(opts.Level)); err != nil {
		return nil, fmt.Errorf("%w: level %q", ErrInvalidOption, opts.Level)
	}

	if !slices.Contains([]string{ContentMessage, ContentLinks, ContentEnvelope}, opts.Content) {
		return nil, fmt.Errorf("%w: content %q", ErrInvalidOption, opts.Content)
	}

	m.content = opts.Content

	return m, nil
}

/*
Send writes the message as one line.
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

	attrs := []log.Attr{
		log.String("mailer", m.name),
		log.Bool("mail_fallback", true),
		log.String("recipients", strings.Join(provider.Emails(recipients), ", ")),
		log.String("subject", msg.Subject),
	}

	if m.content != ContentEnvelope {
		attrs = append(attrs, log.String("links", strings.Join(links(msg), " ")))
	}

	if m.content == ContentMessage {
		attrs = append(attrs, log.String("message", string(raw)))
	}

	m.logger.LogAttrs(ctx, m.level, "mail logged instead of sent", attrs...)

	return mail.SendResult{}, nil
}

/*
links returns every distinct web address in the message, in the order
they appear, without the punctuation of the sentence they end: from the
text body, which every built message has, or from the HTML of a message
without one.
*/
func links(msg mail.Message) []string {
	body := msg.Text
	if body == "" {
		body = html.UnescapeString(msg.HTML)
	}

	var found []string

	for _, link := range linkPattern.FindAllString(body, -1) {
		link = strings.TrimRight(link, ".,;:!?")
		if !slices.Contains(found, link) {
			found = append(found, link)
		}
	}

	return found
}
