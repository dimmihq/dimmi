/*
|--------------------------------------------------------------------------
| Composer
|--------------------------------------------------------------------------
|
| A reply should read like a person wrote it and land in the customer's
| primary inbox: plain HTML with a text twin, no tracking, the previous
| message quoted below. Its headers keep the thread together in every
| client: a Message-ID of our own that names the message it carries,
| In-Reply-To the message it answers, and References from the thread's
| root through its latest messages. A reply token in Reply-To brings
| the answer back even when a client drops all of those.
|
| Automated replies say so (Auto-Submitted), and ask Microsoft clients
| not to answer them with an out-of-office, so two automated systems
| never talk to each other for long. Every message carries the loop
| stamp that tells the install its own mail when it comes back.
|
*/

package email

import (
	"html"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/mail"
)

/*
Shapes of outbound headers: the random part of a Message-ID, and the
longest display name.
*/
const (
	messageIDRandom = 8
	maxDisplayName  = 78
)

/*
Draft is a reply ready to compose.

MessageID, InReplyTo and References are given without angle brackets.
Automated marks mail no person wrote, such as an agent's reply.
LoopStamp and TestStamp are the stamps to carry, when set. Quote is the
previous message to show below the reply, when set. Signing is the key
a relay signs the reply with, for a domain we sign for.
*/
type Draft struct {
	From, ReplyTo mail.Address
	To, Cc        []mail.Address
	Subject       string
	Text, HTML    string
	MessageID     string
	InReplyTo     string
	References    []string
	Automated     bool
	LoopStamp     string
	TestStamp     string
	Quote         *Quote
	Attachments   []mail.Attachment
	Signing       *mail.SigningKey
}

/*
Quote is a previous message shown below a reply: the line that
introduces it and its text.
*/
type Quote struct {
	Header string
	Text   string
}

/*
Compose returns the mail message of the draft.
*/
func Compose(d Draft) mail.Message {
	headers := map[string]string{
		"Message-ID":      "<" + d.MessageID + ">",
		"X-Entity-Ref-ID": NewReplyToken(),
	}

	if d.InReplyTo != "" {
		headers["In-Reply-To"] = "<" + d.InReplyTo + ">"
	}

	if len(d.References) > 0 {
		headers["References"] = "<" + strings.Join(d.References, "> <") + ">"
	}

	if d.Automated {
		headers["Auto-Submitted"] = "auto-replied"
		headers["X-Auto-Response-Suppress"] = "OOF, AutoReply"
	}

	if d.LoopStamp != "" {
		headers[LoopHeader] = d.LoopStamp
	}

	if d.TestStamp != "" {
		headers[TestHeader] = d.TestStamp
	}

	text, body := d.Text, d.HTML
	if d.Quote != nil {
		text += "\n\n" + d.Quote.Header + "\n" + quoteLines(d.Quote.Text)
		body += quoteHTML(*d.Quote)
	}

	return mail.Message{
		From: d.From, ReplyTo: d.ReplyTo, To: d.To, Cc: d.Cc, Subject: d.Subject, Headers: headers,
		Text: text, HTML: wrapHTML(body), Attachments: d.Attachments, Signing: d.Signing,
	}
}

/*
MessageID returns a Message-ID for a message: its public id, a random
part, and the sending domain.
*/
func MessageID(messagePublicID, domain string) string {
	return messagePublicID + "." + NewReplyToken()[:messageIDRandom] + "@" + domain
}

/*
ReplySubject returns the subject of a reply: the conversation's subject
with one Re: in front.
*/
func ReplySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "Re: Your message"
	}

	if HasReplyPrefix(subject) {
		return subject
	}

	return "Re: " + subject
}

/*
TrimReferences keeps a thread's root and its most recent ids, at most
limit in all.
*/
func TrimReferences(refs []string, limit int) []string {
	if len(refs) <= limit || limit < 2 {
		return refs
	}

	return append([]string{refs[0]}, refs[len(refs)-limit+1:]...)
}

/*
DisplayName returns a safe display name: no line breaks, quotes or @,
at most 78 characters.
*/
func DisplayName(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\n', '"', '@', '<', '>':
			return -1
		}

		return r
	}, name)

	name = strings.Join(strings.Fields(name), " ")
	if runes := []rune(name); len(runes) > maxDisplayName {
		name = string(runes[:maxDisplayName])
	}

	return name
}

/*
QuoteHeader returns the line that introduces a quoted message, in the
workspace's time zone.
*/
func QuoteHeader(at time.Time, name, email string, zone *time.Location) string {
	who := email
	if name != "" {
		who = name + " <" + email + ">"
	}

	return "On " + at.In(zone).Format("Mon, 2 Jan 2006 at 15:04") + ", " + who + " wrote:"
}

/*
quoteLines prefixes each line of the quoted text with "> ".
*/
func quoteLines(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight("> "+line, " ")
	}

	return strings.Join(lines, "\n")
}

/*
quoteHTML renders the quoted message below a thin rule.
*/
func quoteHTML(q Quote) string {
	paragraphs := strings.ReplaceAll(html.EscapeString(strings.TrimSpace(q.Text)), "\n", "<br>")

	return `<hr style="border:none;border-top:1px solid #e5e5e5;margin:24px 0 12px">` +
		`<div style="color:#737373">` + html.EscapeString(q.Header) + `</div>` +
		`<blockquote type="cite" style="margin:8px 0 0;padding-left:12px;border-left:2px solid #e5e5e5;` +
		`color:#737373">` + paragraphs + `</blockquote>`
}

/*
wrapHTML sets the reply in the plain look of a personal email.
*/
func wrapHTML(body string) string {
	return `<!doctype html><html><body style="margin:0"><div style="font-family:-apple-system,` +
		`BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;font-size:15px;line-height:1.5;` +
		`max-width:640px">` + body + `</div></body></html>`
}
