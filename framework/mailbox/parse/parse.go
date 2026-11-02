/*
|--------------------------------------------------------------------------
| Message Parsing
|--------------------------------------------------------------------------
|
| Received mail is written by every kind of software, much of it sloppy
| and some of it hostile, so parsing never gives up on a message: broken
| parts become warnings, a document that is not MIME at all becomes its
| own plain text body, and text is always valid, normalized UTF-8.
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
or cannot be read. Truncated is set when the message exceeded MaxBytes
and was only walked.
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
	Attachments []Attachment
	Warnings    []string
	Truncated   bool
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
whole parses a message held in memory, falling back to plain text when
it cannot be read as MIME at all.
*/
func whole(raw []byte, limits Limits) *Parsed {
	parser := enmime.NewParser(enmime.MaxMIMEParts(limits.MaxParts), enmime.SkipMalformedParts(true))

	env, err := parser.ReadEnvelope(bytes.NewReader(raw))
	if err != nil {
		return &Parsed{Text: string(raw), Warnings: []string{"not a mime message: " + err.Error()}}
	}

	parsed := fromHeader(env)
	parsed.Text, parsed.HTML = env.Text, env.HTML

	for _, part := range env.Attachments {
		parsed.Attachments = append(parsed.Attachments, attachment(part, false))
	}

	for _, part := range env.Inlines {
		parsed.Attachments = append(parsed.Attachments, attachment(part, true))
	}

	for _, part := range env.OtherParts {
		parsed.Attachments = append(parsed.Attachments, attachment(part, part.ContentID != ""))
	}

	for _, problem := range env.Errors {
		parsed.Warnings = append(parsed.Warnings, problem.Error())
	}

	return parsed
}

func attachment(part *enmime.Part, inline bool) Attachment {
	return Attachment{
		Filename:    part.FileName,
		ContentType: part.ContentType,
		ContentID:   part.ContentID,
		Inline:      inline,
		Size:        int64(len(part.Content)),
		Content:     part.Content,
	}
}

/*
fromHeader reads the top-level header fields, with encoded words and
addresses decoded.
*/
func fromHeader(env *enmime.Envelope) *Parsed {
	parsed := &Parsed{
		Header:     env.Root.Header,
		ReplyTo:    addresses(env, "Reply-To"),
		To:         addresses(env, "To"),
		Cc:         addresses(env, "Cc"),
		Subject:    env.GetHeader("Subject"),
		InReplyTo:  messageIDs(env.GetHeader("In-Reply-To")),
		References: messageIDs(env.GetHeader("References")),
	}

	if from := addresses(env, "From"); len(from) > 0 {
		parsed.From = from[0]
	}

	if ids := messageIDs(env.GetHeader("Message-ID")); len(ids) > 0 {
		parsed.MessageID = ids[0]
	}

	date, err := env.Date()
	if err == nil {
		parsed.Date = date
	} else if env.GetHeader("Date") != "" {
		parsed.Warnings = append(parsed.Warnings, "unreadable date: "+env.GetHeader("Date"))
	}

	return parsed
}

func addresses(env *enmime.Envelope, field string) []mail.Address {
	list, err := env.AddressList(field)
	if err != nil {
		return nil
	}

	out := make([]mail.Address, 0, len(list))
	for _, a := range list {
		out = append(out, mail.Address{Name: a.Name, Email: a.Address})
	}

	return out
}

/*
headerBlock returns the top-level header of a raw message, ending with
the blank line that closes it.
*/
func headerBlock(raw []byte) []byte {
	for _, end := range [][]byte{[]byte("\r\n\r\n"), []byte("\n\n")} {
		if i := bytes.Index(raw, end); i >= 0 {
			return raw[:i+len(end)]
		}
	}

	return append(raw, "\r\n\r\n"...)
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
