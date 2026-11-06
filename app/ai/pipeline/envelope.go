/*
|--------------------------------------------------------------------------
| Prompt Envelope
|--------------------------------------------------------------------------
|
| Everything that comes from outside — help articles, the customer's
| profile, the conversation, the customer's message — reaches a model
| only inside tagged blocks of the user message, with every &, < and >
| escaped. The only real structure is the envelope itself, so a source
| or a customer cannot close a tag and pose as an instruction.
| Internal notes, events and undone messages never enter it.
|
*/

package pipeline

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Envelope limits: the turns of history sent, the characters each turn
keeps, and the characters of the customer's pending messages.
*/
const (
	historyTurns      = 12
	turnChars         = 1500
	customerChars     = 8000
	attributeChars    = 200
	contactAttributes = 20
)

/*
protocolReminder closes every answer envelope.
*/
const protocolReminder = "Reply per the output protocol: the STATUS line, a blank line, then the reply."

/*
escape makes data inert inside the envelope.
*/
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

/*
escapeAttr makes data inert inside an attribute value.
*/
func escapeAttr(s string) string {
	return strings.ReplaceAll(escape(s), `"`, "&quot;")
}

/*
redactFor masks PII when the agent's setting asks for it.
*/
func (r *Runner) redactFor(e *env, text string) string {
	if !e.Agent.PiiRedaction {
		return text
	}
	return r.redactor.Redact(text)
}

/*
answerEnvelope is the answer model's user message.
*/
func (r *Runner) answerEnvelope(e *env, sources []source) string {
	return strings.Join([]string{
		sourcesBlock(sources),
		r.contactBlock(e),
		r.conversationBlock(e),
		r.customerBlock(e),
		protocolReminder,
	}, "\n")
}

/*
sourcesBlock lists the kept chunks, numbered.
*/
func sourcesBlock(sources []source) string {
	var b strings.Builder
	b.WriteString("<sources>\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "<source id=\"%d\" title=\"%s\" url=\"%s\">%s</source>\n",
			s.n, escapeAttr(s.chunk.Title), escapeAttr(s.chunk.URL), escape(s.chunk.Text))
	}
	b.WriteString("</sources>")
	return b.String()
}

/*
conversationBlock is the history before the pending messages.
*/
func (r *Runner) conversationBlock(e *env) string {
	turns := e.History
	if len(turns) > historyTurns {
		turns = turns[len(turns)-historyTurns:]
	}

	var b strings.Builder
	b.WriteString("<conversation>\n")
	for _, m := range turns {
		text := truncate(r.redactFor(e, body(m)), turnChars)
		fmt.Fprintf(&b, "<turn role=\"%s\">%s</turn>\n", roleOf(m), escape(text))
	}
	b.WriteString("</conversation>")
	return b.String()
}

/*
customerBlock holds every customer message since the last reply,
keeping the newest characters when they run long.
*/
func (r *Runner) customerBlock(e *env) string {
	text := r.redactFor(e, e.pendingText())
	if runes := []rune(text); len(runes) > customerChars {
		text = string(runes[len(runes)-customerChars:])
	}
	return "<customer_message>" + escape(text) + "</customer_message>"
}

/*
contactBlock carries what the privacy setting allows about the
customer, as attributes: the first name and locale, plus allowlisted
attributes when the setting is allowlist.
*/
func (r *Runner) contactBlock(e *env) string {
	if e.Contact == nil || e.Agent.IncludeContactContext == "none" {
		return "<contact></contact>"
	}

	var attrs []string
	if name := firstName(e.Contact); name != "" {
		attrs = append(attrs, attr("name", name))
	}
	if loc := strv(e.Contact.Locale); loc != "" {
		attrs = append(attrs, attr("locale", loc))
	}
	if e.Agent.IncludeContactContext == "allowlist" {
		attrs = append(attrs, r.allowlisted(e)...)
	}

	if len(attrs) == 0 {
		return "<contact></contact>"
	}
	return "<contact " + strings.Join(attrs, " ") + "></contact>"
}

/*
allowlisted renders the contact's allowlisted custom attributes.
*/
func (r *Runner) allowlisted(e *env) []string {
	values := map[string]any{}
	if !decodeJSON(string(e.Contact.CustomAttributes), &values) {
		return nil
	}

	var out []string
	for _, key := range e.Agent.ContactAttributeAllowlist {
		name := attrName().ReplaceAllString(strings.ToLower(key), "_")
		v, ok := values[key]
		if !ok || name == "" || name == "name" || name == "locale" || len(out) >= contactAttributes {
			continue
		}
		text := truncate(r.redactFor(e, fmt.Sprint(v)), attributeChars)
		out = append(out, attr(name, text))
	}
	return out
}

/*
attr renders one escaped attribute.
*/
func attr(name, value string) string {
	return name + `="` + escapeAttr(value) + `"`
}

/*
attrName matches what an attribute name may not contain.
*/
func attrName() *regexp.Regexp {
	return regexp.MustCompile(`[^a-z0-9_]+`)
}

/*
roleOf names a turn's author for the envelope.
*/
func roleOf(m models.Message) string {
	switch m.AuthorType {
	case string(enums.AuthorTypeContact):
		return "customer"
	case string(enums.AuthorTypeMember):
		return "teammate"
	}
	return "assistant"
}

/*
firstName is the contact's first name, at most 40 characters.
*/
func firstName(c *models.Contact) string {
	fields := strings.Fields(strv(c.Name))
	if len(fields) == 0 {
		return ""
	}
	return truncate(fields[0], 40)
}

/*
body is a message's plain text.
*/
func body(m models.Message) string {
	return strings.TrimSpace(strv(m.Body))
}

/*
truncate keeps at most n runes.
*/
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

/*
joinParagraphs joins texts with blank lines.
*/
func joinParagraphs(parts []string) string {
	return strings.Join(parts, "\n\n")
}

/*
strv reads a nullable string.
*/
func strv(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
