/*
|--------------------------------------------------------------------------
| Mailables
|--------------------------------------------------------------------------
|
| A mailable is one kind of mail the application sends, such as a sign-in
| link, written once as a type: its envelope says who it is from and what
| it is about, its content renders the bodies. Senders only pick the
| recipients. Content renders when the mail is sent, so it can read
| current data through the context.
|
|   err := a.Mail.To(mail.Address{Email: user.Email}).Send(ctx, SignInMail{URL: url})
|
| A view renders the HTML body; the text body is derived from it unless
| the content sets one, so every mail carries both.
|
| Mail is written in its recipient's locale, never the sender's: Locale
| names it, and otherwise the locale of the context is kept. Content
| renders with that locale in the context, and may set the subject too,
| so the subject is translated with the body.
|
*/

package mail

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"

	"github.com/k3a/html2text"

	"github.com/dimmihq/dimmi/framework/i18n"
)

/*
Envelope is what a mailable says about itself. An empty From uses the
configured From address. Headers and Tags are copied to the message.
*/
type Envelope struct {
	From, ReplyTo Address
	Subject       string
	Headers       map[string]string
	Tags          map[string]string
}

/*
Renderer writes HTML. Compiled templ components are renderers.
*/
type Renderer interface {
	/*
		Render writes the HTML to the writer.
	*/
	Render(ctx context.Context, w io.Writer) error
}

/*
Content is the body of a mailable: HTML, text, or both, with any
attachments. A Text left empty is derived from the HTML. Subject, when
set, replaces the envelope's subject, so it can be translated in the
locale the content renders in.
*/
type Content struct {
	Subject     string
	Text, HTML  string
	Attachments []Attachment
	view        Renderer
}

/*
View returns content whose HTML body the renderer writes when the mail
is sent.
*/
func View(view Renderer) Content {
	return Content{view: view}
}

/*
Mailable is one kind of mail the application sends.
*/
type Mailable interface {
	/*
		Envelope returns the sender, subject, headers and tags.
	*/
	Envelope() Envelope

	/*
		Content returns the bodies and attachments.
	*/
	Content(ctx context.Context) (Content, error)
}

/*
PendingMail is a mailable being addressed. Start one with Manager.To,
then send it now with Send or later with Queue or QueueTx.
*/
type PendingMail struct {
	manager *Manager
	mailer  string
	locale  string
	to      []Address
	cc      []Address
	bcc     []Address
}

/*
Mailer sends the mailable on the named mailer instead of the default
one, which keeps a stream of mail, such as mail to customers, apart from
the rest. A name missing from the configuration falls back to the
default mailer when the mail is sent.
*/
func (p *PendingMail) Mailer(name string) *PendingMail {
	p.mailer = name

	return p
}

/*
Locale renders the mail in the recipient's locale, such as "fr", rather
than the locale of the context it is sent or queued from.
*/
func (p *PendingMail) Locale(locale string) *PendingMail {
	p.locale = locale

	return p
}

/*
Cc adds visible copy recipients.
*/
func (p *PendingMail) Cc(recipients ...Address) *PendingMail {
	p.cc = append(p.cc, recipients...)

	return p
}

/*
Bcc adds hidden copy recipients.
*/
func (p *PendingMail) Bcc(recipients ...Address) *PendingMail {
	p.bcc = append(p.bcc, recipients...)

	return p
}

/*
Send renders the mailable and sends it now, on the mailer chosen with
Mailer or else the default one.
*/
func (p *PendingMail) Send(ctx context.Context, mailable Mailable) (SendResult, error) {
	return p.sendOn(ctx, p.mailerName(), mailable)
}

/*
mailerName returns the mailer the mail goes out on: the one chosen with
Mailer, or the default.
*/
func (p *PendingMail) mailerName() string {
	return cmp.Or(p.mailer, p.manager.mailers.DefaultName())
}

func (p *PendingMail) sendOn(ctx context.Context, name string, mailable Mailable) (SendResult, error) {
	ctx = p.localized(ctx)

	msg, err := p.message(ctx, mailable)
	if err != nil {
		return SendResult{}, err
	}

	if fake := p.manager.fake.Load(); fake != nil {
		return fake.record(ctx, name, msg, mailable)
	}

	mailer, err := p.manager.Mailer(name)
	if err != nil {
		return SendResult{}, err
	}

	return mailer.Send(ctx, msg)
}

/*
localized returns the context the mail renders in: the mail's own
locale when it has one, and the manager's translator when it has one.
*/
func (p *PendingMail) localized(ctx context.Context) context.Context {
	if p.locale != "" {
		ctx = i18n.WithLocale(ctx, p.locale)
	}

	if p.manager.translator != nil {
		ctx = i18n.WithTranslator(ctx, p.manager.translator)
	}

	return ctx
}

func (p *PendingMail) message(ctx context.Context, mailable Mailable) (Message, error) {
	envelope := mailable.Envelope()

	content, err := mailable.Content(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("mailable content: %w", err)
	}

	if err := content.render(ctx); err != nil {
		return Message{}, err
	}

	from := envelope.From
	if from.Email == "" {
		from = p.manager.from
	}

	return Message{
		From:        from,
		ReplyTo:     envelope.ReplyTo,
		To:          p.to,
		Cc:          p.cc,
		Bcc:         p.bcc,
		Subject:     cmp.Or(content.Subject, envelope.Subject),
		Text:        content.Text,
		HTML:        content.HTML,
		Headers:     maps.Clone(envelope.Headers),
		Attachments: content.Attachments,
		Tags:        maps.Clone(envelope.Tags),
	}, nil
}

/*
render writes the view into the HTML body and derives the text body
when none is set.
*/
func (c *Content) render(ctx context.Context) error {
	if c.view != nil {
		var html bytes.Buffer
		if err := c.view.Render(ctx, &html); err != nil {
			return fmt.Errorf("render mail view: %w", err)
		}

		c.HTML = html.String()
	}

	if c.Text == "" && c.HTML != "" {
		c.Text = html2text.HTML2TextWithOptions(c.HTML, html2text.WithUnixLineBreaks(), html2text.WithLinksInnerText())
	}

	return nil
}
