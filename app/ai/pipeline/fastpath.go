/*
|--------------------------------------------------------------------------
| Fast Paths
|--------------------------------------------------------------------------
|
| Some messages never deserve a model call: "human", "thanks", "yes".
| Each pattern answers instantly, deterministically and for free.
|
*/

package pipeline

import (
	"regexp"
	"strings"
)

/*
fastPath decides instantly from the message's own words. The order is
deliberate: asking for a person beats everything, and a confirmation
only counts while one is pending.
*/
func fastPath(text string, pendingConfirmation bool) (outcome string, handoffReason string) {
	t := normalize(text)

	switch {
	case asksHuman(t):
		return "handoff", "customer_requested"
	case isAbuse(t):
		return "handoff", "abusive"
	case isAcknowledgement(t):
		return "silent", ""
	case pendingConfirmation && isConfirmation(t):
		return "confirm", ""
	case pendingConfirmation && isCancellation(t):
		return "cancel", ""
	}
	return "", ""
}

/*
humanPatterns match asking for a person.
*/
var humanPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(human|agent|person|operator|staff|representative|real person|support person|someone real)\b`),
	regexp.MustCompile(`\b(talk|speak|connect|transfer|escalate).{0,20}\b(someone|somebody|anyone|person|agent|human|team)\b`),
	regexp.MustCompile(`\bget me (a|an|the)? ?(human|agent|person)\b`),
}

/*
ackPatterns match bare acknowledgements.
*/
var ackPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(thanks?|thank you|thx|ty|ok|okay|got it|perfect|great|cool|nice|awesome|understood|makes sense|that helps?|helpful|solved|done|never ?mind|no worries|all good|👍|🙏|✅)[\s!.]*$`),
}

/*
abusePatterns match the crudest attacks.
*/
var abusePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(shut up|fuck you|fuck off|stupid bot|dumb bot|useless bot|hate you|kill yourself)\b`),
}

/*
confirmPatterns match yes-answers.
*/
var confirmPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(yes|yeah|yep|yup|sure|confirm|confirmed|do it|go ahead|ok|okay|please do|correct|that's right|right|y)[\s!.]*$`),
}

/*
cancelPatterns match no-answers.
*/
var cancelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(no|nope|cancel|cancelled|don't|do not|stop|not now|never mind|n)[\s!.]*$`),
}

/*
asksHuman reports whether the text asks for a person.
*/
func asksHuman(t string) bool {
	for _, p := range humanPatterns {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isAcknowledgement reports whether the text only acknowledges.
*/
func isAcknowledgement(t string) bool {
	if len(t) > 60 {
		return false
	}
	for _, p := range ackPatterns {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isAbuse reports whether the text is abusive.
*/
func isAbuse(t string) bool {
	for _, p := range abusePatterns {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isConfirmation reports whether the text confirms.
*/
func isConfirmation(t string) bool {
	for _, p := range confirmPatterns {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isCancellation reports whether the text declines.
*/
func isCancellation(t string) bool {
	for _, p := range cancelPatterns {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
normalize prepares text for matching.
*/
func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
