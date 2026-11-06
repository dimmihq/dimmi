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
patterns holds the fast path matchers, built once per runner.
*/
type patterns struct {
	human, ack, abuse, confirm, cancel []*regexp.Regexp
}

/*
newPatterns compiles the matchers.
*/
func newPatterns() patterns {
	return patterns{
		human: []*regexp.Regexp{
			regexp.MustCompile(`\b(human|agent|person|operator|staff|representative|real person|support person|someone real)\b`),
			regexp.MustCompile(`\b(talk|speak|connect|transfer|escalate).{0,20}\b(someone|somebody|anyone|person|agent|human|team)\b`),
			regexp.MustCompile(`\bget me (a|an|the)? ?(human|agent|person)\b`),
		},
		ack: []*regexp.Regexp{
			regexp.MustCompile(`^(thanks?|thank you|thx|ty|ok|okay|got it|perfect|great|cool|nice|awesome|understood|makes sense|that helps?|helpful|solved|done|never ?mind|no worries|all good|👍|🙏|✅)[\s!.]*$`),
		},
		abuse: []*regexp.Regexp{
			regexp.MustCompile(`\b(shut up|fuck you|fuck off|stupid bot|dumb bot|useless bot|hate you|kill yourself)\b`),
		},
		confirm: []*regexp.Regexp{
			regexp.MustCompile(`^(yes|yeah|yep|yup|sure|confirm|confirmed|do it|go ahead|ok|okay|please do|correct|that's right|right|y)[\s!.]*$`),
		},
		cancel: []*regexp.Regexp{
			regexp.MustCompile(`^(no|nope|cancel|canceled|don't|do not|stop|not now|never mind|n)[\s!.]*$`),
		},
	}
}

/*
fastPath decides instantly from the message's own words. The order is
deliberate: asking for a person beats everything, and a confirmation
only counts while one is pending.
*/
func (p patterns) fastPath(text string, pendingConfirmation bool) (outcome, handoffReason string) {
	t := normalize(text)

	switch {
	case p.asksHuman(t):
		return "handoff", "customer_requested"
	case p.isAbuse(t):
		return "handoff", "abusive"
	case p.isAcknowledgement(t):
		return "silent", ""
	case pendingConfirmation && p.isConfirmation(t):
		return "confirm", ""
	case pendingConfirmation && p.isCancellation(t):
		return "cancel", ""
	}
	return "", ""
}

/*
asksHuman reports whether the text asks for a person.
*/
func (p patterns) asksHuman(t string) bool {
	for _, re := range p.human {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isAcknowledgement reports whether the text only acknowledges.
*/
func (p patterns) isAcknowledgement(t string) bool {
	if len(t) > 60 {
		return false
	}
	for _, re := range p.ack {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isAbuse reports whether the text is abusive.
*/
func (p patterns) isAbuse(t string) bool {
	for _, re := range p.abuse {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isConfirmation reports whether the text confirms.
*/
func (p patterns) isConfirmation(t string) bool {
	for _, re := range p.confirm {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}

/*
isCancellation reports whether the text declines.
*/
func (p patterns) isCancellation(t string) bool {
	for _, re := range p.cancel {
		if re.MatchString(t) {
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
