/*
|--------------------------------------------------------------------------
| Forwarded Mail
|--------------------------------------------------------------------------
|
| Teammates forward customer mail to the support address. The message
| then comes from the teammate, but the conversation belongs to the
| customer whose mail is inside, so the forwarded block is found and its
| sender, subject and body read out. Every common client is recognized:
| Gmail, Apple Mail, Thunderbird and Outlook in several languages. What
| the teammate wrote above the block is their comment.
|
*/

package email

import (
	"regexp"
	"strings"
)

/*
maxForwardHeaderLines bounds how far below the marker the forwarded
header fields are looked for.
*/
const maxForwardHeaderLines = 8

var (
	forwardMarker = regexp.MustCompile(`(?i)^(-{2,}\s*forwarded message\s*-{2,}|begin forwarded message:?|` +
		`-{2,}\s*(weitergeleitete nachricht|message transféré|mensaje reenviado|messaggio inoltrato|` +
		`doorgestuurd bericht)\s*-{2,}|_{20,})$`)
	forwardField = regexp.MustCompile(`(?i)^\**(from|von|de|da|van|från|subject|betreff|objet|asunto|oggetto|` +
		`onderwerp|ämne|date|sent|gesendet|envoyé|enviado|inviato|verzonden|datum|to|an|à|para|a|aan|cc)` +
		`\s*:\**\s*(.*)$`)
	markdownMailto = regexp.MustCompile(`\[([^\]]*)\]\(mailto:[^)]*\)`)
	emailPattern   = regexp.MustCompile(`[^\s<>"'(),;:\[\]]+@[^\s<>"'(),;:\[\]]+\.[a-zA-Z]{2,}`)
)

/*
Forward is a forwarded message found inside a body.

Name and Email are its original sender, Email empty when the forward
lost the address. Comment is what the forwarder wrote above it.
*/
type Forward struct {
	Name    string
	Email   string
	Subject string
	Comment string
	Body    string
}

/*
FindForward looks for a forwarded block in a plain text or Markdown body
and reads it.
*/
func FindForward(text string) (Forward, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	for i, line := range lines {
		if !forwardMarker.MatchString(plainLine(line)) {
			continue
		}

		forward, end, ok := forwardHeader(lines, i+1)
		if !ok {
			continue
		}

		forward.Comment = strings.TrimSpace(strings.Join(lines[:i], "\n"))
		forward.Body = strings.TrimSpace(strings.Join(lines[end:], "\n"))

		return forward, true
	}

	return Forward{}, false
}

/*
forwardHeader reads the header fields of a forwarded block starting at
line start, and returns where its body begins. A block needs at least a
From field.
*/
func forwardHeader(lines []string, start int) (Forward, int, bool) {
	var forward Forward

	from, end := false, start

	for i := start; i < len(lines) && i < start+maxForwardHeaderLines; i++ {
		line := plainLine(lines[i])
		if line == "" && from {
			return forward, i + 1, true
		}

		match := forwardField.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		end = i + 1
		switch strings.ToLower(match[1]) {
		case "from", "von", "de", "da", "van", "från":
			forward.Name, forward.Email, from = senderOf(match[2])
		case "subject", "betreff", "objet", "asunto", "oggetto", "onderwerp", "ämne":
			forward.Subject = strings.TrimSpace(match[2])
		}
	}

	return forward, end, from
}

/*
senderOf reads a From field written for people: a name and an address
in angle brackets, a bare address, or only a name. It always reports a
sender, since the field was there.
*/
func senderOf(field string) (string, string, bool) {
	field = strings.ReplaceAll(markdownMailto.ReplaceAllString(field, "$1"), "*", "")
	email := strings.ToLower(emailPattern.FindString(field))

	name := field
	if i := strings.Index(name, "<"); i >= 0 {
		name = name[:i]
	}

	if email != "" {
		name = strings.ReplaceAll(name, email, "")
	}

	return strings.Trim(strings.TrimSpace(name), `"'`), email, true
}
