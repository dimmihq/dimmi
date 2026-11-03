/*
|--------------------------------------------------------------------------
| Subjects
|--------------------------------------------------------------------------
|
| A reply's subject grows prefixes on the way: Re, Fwd, AW, SV, 回复,
| sometimes counted, sometimes with a full-width colon, plus tags a
| company's mail gateway adds such as [EXTERNAL]. Normalizing strips
| them all and folds case, so two subjects of one thread compare equal.
|
| Comparing subjects is only a last resort for threading, and only for
| subjects specific enough to mean something: "Help" from two customers
| is two conversations.
|
*/

package email

import (
	"regexp"
	"strings"
)

/*
maxSubjectLength is the longest subject a conversation keeps.
*/
const maxSubjectLength = 255

/*
minHeuristicSubject is the shortest normalized subject the subject
heuristic trusts.
*/
const minHeuristicSubject = 8

var (
	replyPrefix = regexp.MustCompile(
		`^(?i)\s*(re|fw|fwd|aw|wg|sv|vs|antw|r|rif|tr|enc|res|odp|pd|ynt|ilt|回复|转发|答复)` +
			`\s*(\[\d+\]|\(\d+\))?\s*[:：]\s*`)
	gatewayTag = regexp.MustCompile(`(?i)\[\s*(external|ext|spam|suspicious|#\d+)\s*\]`)
	spaces     = regexp.MustCompile(`\s+`)
)

/*
NormalizeSubject returns the subject without reply and forward prefixes
or gateway tags, with whitespace collapsed and case folded.
*/
func NormalizeSubject(subject string) string {
	return strings.ToLower(stripSubject(subject))
}

/*
HasReplyPrefix reports whether the subject starts like a reply or a
forward.
*/
func HasReplyPrefix(subject string) bool {
	return replyPrefix.MatchString(gatewayTag.ReplaceAllString(subject, " "))
}

/*
DisplaySubject returns the subject a new conversation shows: prefixes
removed but the writer's own casing kept, at most 255 characters.
*/
func DisplaySubject(subject string) string {
	s := stripSubject(subject)
	if runes := []rune(s); len(runes) > maxSubjectLength {
		s = string(runes[:maxSubjectLength])
	}

	return s
}

/*
SpecificSubject reports whether a normalized subject says enough to
thread on: long enough and not one of the words everyone uses.
*/
func SpecificSubject(normalized string) bool {
	if len([]rune(normalized)) < minHeuristicSubject {
		return false
	}

	switch normalized {
	case "hello", "hi", "help", "question", "support", "(no subject)", "no subject", "inquiry", "issue",
		"problem", "test", "urgent", "hilfe", "frage", "aide", "ayuda", "pregunta", "domanda", "vraag",
		"help needed", "need help", "quick question":
		return false
	}

	return true
}

/*
stripSubject removes gateway tags and every leading reply or forward
prefix, then collapses whitespace.
*/
func stripSubject(subject string) string {
	s := gatewayTag.ReplaceAllString(subject, " ")
	for {
		stripped := replyPrefix.ReplaceAllString(s, "")
		if stripped == s {
			break
		}

		s = stripped
	}

	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}
