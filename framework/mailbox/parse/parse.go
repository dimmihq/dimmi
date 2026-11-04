/*
|--------------------------------------------------------------------------
| Message Parsing
|--------------------------------------------------------------------------
|
| Received mail is written by every kind of software, much of it sloppy
| and some of it hostile, so parsing never gives up on a message: broken
| parts become warnings, a body that cannot be read as MIME becomes plain
| text under the header it still has, a document that is not mail at all
| becomes its own plain text body, and text is always valid, normalized
| UTF-8.
|
| Memory stays bounded. A message within the size limit is parsed whole.
| A larger one is walked as a stream instead: its headers and the first
| text and HTML bodies are kept up to a cap each, and its attachments
| are only listed with their sizes, marked as dropped.
|
*/

package parse

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	netmail "net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/jhillyerd/enmime/v2"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
Defaults for every zero field of Limits.
*/
const (
	DefaultMaxBytes     = 30 << 20
	DefaultMaxPartBytes = 1 << 20
	DefaultMaxParts     = 1000
	DefaultMaxWarnings  = 10
)

/*
Limits bound the work one message may cause.

MaxBytes is the largest message parsed whole. MaxPartBytes caps each
body kept from a larger message. MaxParts caps the MIME parts of a
message parsed whole; one with more is read as plain text. MaxWarnings
caps the warnings kept.
*/
type Limits struct {
	MaxBytes     int64
	MaxPartBytes int64
	MaxParts     int
	MaxWarnings  int
}

/*
Attachment is a file in a message. A dropped attachment was too large
to keep: only its name, type and size are known.
*/
type Attachment struct {
	Filename    string
	ContentType string
	ContentID   string
	Inline      bool
	Size        int64
	Content     []byte
	Dropped     bool
}

/*
Parsed is a received message.

Header holds every top-level header field as received. Message ids are
given without angle brackets. Date is zero when the header is missing
or cannot be read. TextDerived is set when the message had no text of
its own and Text was converted from its HTML. Truncated is set when the
message exceeded MaxBytes and was only walked.
*/
type Parsed struct {
	Header      textproto.MIMEHeader
	From        mail.Address
	ReplyTo     []mail.Address
	To, Cc      []mail.Address
	Subject     string
	Date        time.Time
	MessageID   string
	InReplyTo   []string
	References  []string
	Text, HTML  string
	TextDerived bool
	Attachments []Attachment
	Warnings    []string
	Truncated   bool
}

/*
TextPart returns the text the message itself carried, without a
conversion from its HTML. A fallback reads this rather than Text, since
a conversion brings back what the HTML hides from a reader.
*/
func (p *Parsed) TextPart() string {
	if p.TextDerived {
		return ""
	}

	return p.Text
}

/*
Message parses the raw message within the limits. It fails only when
the message cannot be read.
*/
func Message(raw io.Reader, limits Limits) (*Parsed, error) {
	limits.MaxBytes = cmp.Or(limits.MaxBytes, DefaultMaxBytes)
	limits.MaxPartBytes = cmp.Or(limits.MaxPartBytes, DefaultMaxPartBytes)
	limits.MaxParts = cmp.Or(limits.MaxParts, DefaultMaxParts)
	limits.MaxWarnings = cmp.Or(limits.MaxWarnings, DefaultMaxWarnings)

	head, err := io.ReadAll(io.LimitReader(raw, limits.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read message: %w", err)
	}

	var parsed *Parsed
	if int64(len(head)) > limits.MaxBytes {
		parsed, err = walk(head, raw, limits)
	} else {
		parsed = whole(head, limits)
	}

	if err != nil {
		return nil, err
	}

	parsed.normalize(limits)

	return parsed, nil
}

/*
whole parses a message held in memory. Its header is read whatever state
the rest is in, and a body the parser cannot make sense of becomes the
message's plain text.
*/
func whole(raw []byte, limits Limits) *Parsed {
	top, ok := readHead(raw)
	if !ok {
		return &Parsed{Text: decodeText(raw, ""), Warnings: []string{"not a mime message"}}
	}

	parsed := fromHeader(top)
	parser := enmime.NewParser(
		enmime.MaxMIMEParts(limits.MaxParts), enmime.SkipMalformedParts(true), enmime.DisableTextConversion(true),
		enmime.DisableCharacterDetection(true), enmime.SetCustomParseMediaType(mediaTypeReader()),
		enmime.MultipartWOBoundaryAsSinglePart(true),
	)

	env, err := parser.ReadEnvelope(bytes.NewReader(top.message))
	if err != nil {
		parsed.Text = plainBody(top)
		parsed.Warnings = append(parsed.Warnings, "unreadable structure: "+err.Error())

		return parsed
	}

	broken := readBodies(parsed, env)

	for _, problem := range env.Errors {
		parsed.Warnings = append(parsed.Warnings, problem.Error())
	}

	if (parsed.empty() || (broken && parsed.wordless())) && len(bytes.TrimSpace(top.body)) > 0 {
		parsed.Text = plainBody(top)
		parsed.Warnings = append(parsed.Warnings, "unreadable structure: body read as plain text")
	}

	return parsed
}

/*
plainBody reads a body that makes no sense as MIME as plain text, still
undoing the transfer encoding the header names.
*/
func plainBody(top head) string {
	encoding := strings.TrimSpace(top.fields.Get("Content-Transfer-Encoding"))
	if encoding == "" || strings.ContainsAny(encoding, "\r\n") {
		return decodeText(top.body, "")
	}

	header := "Content-Transfer-Encoding: " + encoding + "\r\nContent-Type: text/plain\r\n\r\n"
	message := append([]byte(header), top.body...)

	root, err := enmime.NewParser().ReadParts(bytes.NewReader(message))
	if err != nil {
		return decodeText(top.body, "")
	}

	return decodeText(root.Content, "")
}

/*
empty reports whether nothing was read from the message below its header.
*/
func (p *Parsed) empty() bool {
	return p.wordless() && len(p.Attachments) == 0
}

/*
wordless reports whether no body was read from the message.
*/
func (p *Parsed) wordless() bool {
	return strings.TrimSpace(p.Text) == "" && strings.TrimSpace(p.HTML) == ""
}

/*
fromHeader reads the top-level header fields, with encoded words and
addresses decoded.
*/
func fromHeader(top head) *Parsed {
	parsed := &Parsed{
		Header:     top.fields,
		From:       readSender(top.fields, top.message),
		ReplyTo:    readList(top.fields, "Reply-To", top.message),
		To:         readList(top.fields, "To", top.message),
		Cc:         readList(top.fields, "Cc", top.message),
		Subject:    decodeWords(repairHeader(top.fields.Get("Subject"), top.message)),
		InReplyTo:  messageIDs(top.fields.Get("In-Reply-To")),
		References: messageIDs(top.fields.Get("References")),
	}

	if ids := messageIDs(top.fields.Get("Message-Id")); len(ids) > 0 {
		parsed.MessageID = ids[0]
	}

	date := top.fields.Get("Date")
	if date == "" {
		return parsed
	}

	when, err := netmail.ParseDate(date)
	if err != nil {
		parsed.Warnings = append(parsed.Warnings, "unreadable date: "+date)

		return parsed
	}

	parsed.Date = when

	return parsed
}

/*
messageIDs reads a list of message ids, with or without angle brackets.
*/
func messageIDs(text string) []string {
	var ids []string
	for field := range strings.FieldsFuncSeq(text, func(r rune) bool { return r == ' ' || r == ',' || r < ' ' }) {
		id := strings.Trim(field, "<>")
		if id == "" || len(id) > maxIDLength {
			continue
		}

		ids = append(ids, id)
		if len(ids) == maxIDs {
			break
		}
	}

	return ids
}
