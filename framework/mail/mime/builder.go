/*
|--------------------------------------------------------------------------
| MIME Builder
|--------------------------------------------------------------------------
|
| Drivers that hand a provider a whole document, and callers that build
| a message once to sign or store it, build it here. The structure
| follows the content: alternative text and HTML bodies, related inline
| images, mixed attachments.
|
| The document names nothing about the software that built it, Bcc
| recipients never appear in it, and every message gets a Message-ID on
| its sender's domain unless the caller set one, so replies can be
| matched to it.
|
*/

package mime

import (
	"bytes"
	"crypto/rand"
	"fmt"
	netmail "net/mail"
	"net/textproto"
	"strings"

	gomail "github.com/wneessen/go-mail"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
Builder writes messages as MIME documents. It is safe for concurrent
use.
*/
type Builder struct {
	clock clock.Clock
}

/*
NewBuilder returns a builder that dates messages by the clock.
*/
func NewBuilder(clk clock.Clock) *Builder {
	return &Builder{clock: clk}
}

/*
Build returns the message as a MIME document, or its raw document as it
is when set. The message must be valid.
*/
func (b *Builder) Build(msg mail.Message) ([]byte, error) {
	if err := msg.Validate(); err != nil {
		return nil, err
	}

	if len(msg.Raw) > 0 {
		return msg.Raw, nil
	}

	m := gomail.NewMsg(gomail.WithNoDefaultUserAgent())
	m.FromMailAddress(address(msg.From))

	if len(msg.To) > 0 {
		m.ToMailAddress(addresses(msg.To)...)
	}

	if len(msg.Cc) > 0 {
		m.CcMailAddress(addresses(msg.Cc)...)
	}

	if msg.ReplyTo.Email != "" {
		m.ReplyToMailAddress(address(msg.ReplyTo))
	}

	m.Subject(msg.Subject)
	m.SetDateWithValue(b.clock.Now())
	setHeaders(m, msg)
	setBody(m, msg)

	if err := attach(m, msg.Attachments); err != nil {
		return nil, err
	}

	var raw bytes.Buffer
	if _, err := m.WriteTo(&raw); err != nil {
		return nil, fmt.Errorf("write mime message: %w", err)
	}

	return raw.Bytes(), nil
}

/*
setHeaders copies the extra headers and adds a Message-ID on the
sender's domain when there is none.
*/
func setHeaders(m *gomail.Msg, msg mail.Message) {
	hasID := false

	for name, value := range msg.Headers {
		header := gomail.Header(textproto.CanonicalMIMEHeaderKey(name))
		if strings.EqualFold(name, string(gomail.HeaderMessageID)) {
			header, hasID = gomail.HeaderMessageID, true
		}

		m.SetGenHeader(header, value)
	}

	if !hasID {
		_, domain, _ := strings.Cut(msg.From.Email, "@")
		m.SetMessageIDWithValue(strings.ToLower(rand.Text()) + "@" + domain)
	}
}

func setBody(m *gomail.Msg, msg mail.Message) {
	switch {
	case msg.Text != "" && msg.HTML != "":
		m.SetBodyString(gomail.TypeTextPlain, msg.Text)
		m.AddAlternativeString(gomail.TypeTextHTML, msg.HTML)
	case msg.HTML != "":
		m.SetBodyString(gomail.TypeTextHTML, msg.HTML)
	default:
		m.SetBodyString(gomail.TypeTextPlain, msg.Text)
	}
}

func attach(m *gomail.Msg, attachments []mail.Attachment) error {
	for _, file := range attachments {
		opts := []gomail.FileOption{}
		if file.ContentType != "" {
			opts = append(opts, gomail.WithFileContentType(gomail.ContentType(file.ContentType)))
		}

		add := m.AttachReader
		if file.Inline {
			add = m.EmbedReader
			opts = append(opts, gomail.WithFileContentID("<"+file.ContentID+">"))
		}

		if err := add(file.Filename, bytes.NewReader(file.Content), opts...); err != nil {
			return fmt.Errorf("attach %s: %w", file.Filename, err)
		}
	}

	return nil
}

func address(a mail.Address) *netmail.Address {
	return &netmail.Address{Name: a.Name, Address: a.Email}
}

func addresses(list []mail.Address) []*netmail.Address {
	out := make([]*netmail.Address, 0, len(list))
	for _, a := range list {
		out = append(out, address(a))
	}

	return out
}
