/*
|--------------------------------------------------------------------------
| Conversation Envelope
|--------------------------------------------------------------------------
|
| The answer model reads the conversation as chat turns. Everything a
| person outside wrote stays inside a tag with &, < and > escaped: a
| customer's words in <customer_message>, a teammate's reply in
| <teammate_message>, both as user turns; the agent's own replies are
| its assistant turns. What the workspace allows the agent to know
| about the customer opens the first user turn as <contact>. Internal
| notes, events and undone messages never enter it.
|
*/

package pipeline

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Envelope limits: the turns of history sent, the characters each turn
keeps, the characters of the customer's pending messages, and how much
of the contact's allowlisted attributes is sent.
*/
const (
	historyTurns      = 12
	turnChars         = 1500
	customerChars     = 8000
	attributeChars    = 200
	contactAttributes = 20
)

/*
escape makes data inert inside the envelope.
*/
func escape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
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
conversationMessages are the turns the answer model reads: the recent
history and, last, the customer's messages since the last reply.
*/
func (r *Runner) conversationMessages(e *env) []contracts.ChatMessage {
	return r.envelope(e, "")
}

/*
envelope builds the turns. An update from the system, such as the news
on an action request, comes last as its own user turn; with one, an
empty customer turn is left out. The last turn carries today's date, so
the agent can check dates against a policy without it ever reaching the
cached system prompt.
*/
func (r *Runner) envelope(e *env, update string) []contracts.ChatMessage {
	msgs := r.historyTurns(e)

	pending := r.redactFor(e, e.pendingText())
	if runes := []rune(pending); len(runes) > customerChars {
		pending = string(runes[len(runes)-customerChars:])
	}
	if pending != "" || update == "" {
		msgs = addTurn(msgs, contracts.ChatRoleUser, "<customer_message>"+escape(pending)+"</customer_message>")
	}
	if update != "" {
		msgs = addTurn(msgs, contracts.ChatRoleUser, update)
	}
	last := &msgs[len(msgs)-1]
	last.Parts[0].Text += "\n<today>" + e.now.UTC().Format("Monday 2 January 2006") + " (UTC)</today>" +
		incidentBlock(e)

	if contact := r.contactBlock(e); contact != "" {
		if speaker(msgs[0]) != contracts.ChatRoleUser {
			msgs = append([]contracts.ChatMessage{userMessage(contact)}, msgs...)
		} else {
			msgs[0].Parts[0].Text = contact + "\n" + msgs[0].Parts[0].Text
		}
	}
	if speaker(msgs[0]) != contracts.ChatRoleUser {
		msgs = append([]contracts.ChatMessage{userMessage("<conversation_start></conversation_start>")}, msgs...)
	}
	return msgs
}

/*
historyTurns are the recent turns before the pending messages.
*/
func (r *Runner) historyTurns(e *env) []contracts.ChatMessage {
	history := e.History
	if len(history) > historyTurns {
		history = history[len(history)-historyTurns:]
	}

	var msgs []contracts.ChatMessage
	for _, m := range history {
		text := truncate(r.redactFor(e, body(m)), turnChars)
		switch m.AuthorType {
		case string(enums.AuthorTypeContact):
			msgs = addTurn(msgs, contracts.ChatRoleUser, "<customer_message>"+escape(text)+"</customer_message>")
		case string(enums.AuthorTypeMember):
			msgs = addTurn(msgs, contracts.ChatRoleUser, "<teammate_message>"+escape(text)+"</teammate_message>")
		default:
			msgs = addTurn(msgs, contracts.ChatRoleAssistant, text)
		}
	}
	return msgs
}

/*
addTurn appends a turn, joining it to the previous one when the same
side spoke twice in a row, since providers want turns to alternate.
*/
func addTurn(msgs []contracts.ChatMessage, role contracts.ChatRole, text string) []contracts.ChatMessage {
	if n := len(msgs); n > 0 && speaker(msgs[n-1]) == role && len(msgs[n-1].Parts) > 0 {
		msgs[n-1].Parts[0].Text += "\n" + text
		return msgs
	}
	return append(msgs, contracts.ChatMessage{Role: role, Parts: []contracts.ContentPart{{Text: text}}})
}

/*
speaker is who spoke a turn.
*/
func speaker(m contracts.ChatMessage) contracts.ChatRole {
	return m.Role
}

/*
contactBlock carries what the privacy setting allows about the
customer, as attributes: the first name and locale, plus allowlisted
attributes when the setting is allowlist. Empty when nothing may be
said.
*/
func (r *Runner) contactBlock(e *env) string {
	if e.Contact == nil || e.Agent.IncludeContactContext == "none" {
		return ""
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
		return ""
	}
	return "<contact " + strings.Join(attrs, " ") + "></contact>"
}

/*
allowlisted renders the contact's allowlisted custom attributes.
*/
func (r *Runner) allowlisted(e *env) []string {
	values := map[string]any{}
	if json.Unmarshal(e.Contact.CustomAttributes, &values) != nil {
		return nil
	}

	var out []string
	for _, key := range e.Agent.ContactAttributeAllowlist {
		name := attrName().ReplaceAllString(strings.ToLower(key), "_")
		v, ok := values[key]
		if !ok || name == "" || name == "name" || name == "locale" || len(out) >= contactAttributes {
			continue
		}
		out = append(out, attr(name, truncate(r.redactFor(e, fmt.Sprint(v)), attributeChars)))
	}
	return out
}

/*
attr renders one escaped attribute.
*/
func attr(name, value string) string {
	return name + `="` + strings.ReplaceAll(escape(value), `"`, "&quot;") + `"`
}

/*
attrName matches what an attribute name may not contain.
*/
func attrName() *regexp.Regexp {
	return regexp.MustCompile(`[^a-z0-9_]+`)
}

/*
userMessage is one user turn.
*/
func userMessage(text string) contracts.ChatMessage {
	return contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: text}}}
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
