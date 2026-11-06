/*
|--------------------------------------------------------------------------
| Human Fast Path
|--------------------------------------------------------------------------
|
| A customer who plainly asks for a person gets one, at once, without a
| model call. "Plainly" is narrow on purpose: "talk to a human",
| "real person", "operator" alone, and the same requests in the
| languages customers write most. A sentence that merely mentions a
| staff member or what the agent supports is a question, and goes to
| the classifier, which catches the paraphrases.
|
*/

package pipeline

import (
	"regexp"
	"strings"
	"unicode"
)

/*
fastPath hands off on an explicit request for a person.
*/
func (r *Runner) fastPath(st *run) *decision {
	if !AsksForHuman(st.env.pendingText()) {
		return nil
	}
	st.c = classification{Intent: "human_request", WantsHuman: true}
	return handoff("customer_requested", "", st.env.handoffKey())
}

/*
AsksForHuman reports whether the text explicitly asks for a person.
*/
func AsksForHuman(text string) bool {
	t := normalize(text)
	if t == "" {
		return false
	}
	for _, p := range humanPatterns() {
		if p.MatchString(t) {
			return true
		}
	}
	return false
}

/*
humanPatterns are the explicit requests, matched on normalized text:
a whole message that is only the word, or a request phrase anywhere.
*/
func humanPatterns() []*regexp.Regexp {
	alone := `^(please )?(a )?(human|operator|agent|representative|real person|person please|human please` +
		`|mensch|menschen|mitarbeiter|humain|conseiller|humano|atendente|operatore|umano|mens|medewerker` +
		`|人間|オペレーター|担当者)( please| bitte| por favor| s il vous plait| per favore| alstublieft)?$`
	return []*regexp.Regexp{
		regexp.MustCompile(alone),
		regexp.MustCompile(`\b(talk|speak|chat) (to|with) (a |an |the )?(real )?(human|person|agent|someone` +
			`|somebody|representative|operator|support person|support agent)\b`),
		regexp.MustCompile(`\b(connect|transfer|put) me (to|with|through to) (a |an |the )?(real )?(human|person` +
			`|agent|someone|somebody|representative|operator)\b`),
		regexp.MustCompile(`\b(get|give) me (a |an )?(real )?(human|person|representative|operator)\b`),
		regexp.MustCompile(`\b(a|an)? ?real (human|person)\b`),
		regexp.MustCompile(`\b(mit )?(einem |einer )?(menschen|mitarbeiter|mitarbeiterin|echten person)` +
			` (sprechen|reden|chatten)\b`),
		regexp.MustCompile(`\bparler (a|avec) (un |une )?(humain|conseiller|conseillere|agent|quelqu un` +
			`|vraie personne|personne reelle)\b`),
		regexp.MustCompile(`\bhablar con (un |una )?(humano|persona|agente|alguien|persona real)\b`),
		regexp.MustCompile(`\bfalar com (um |uma )?(humano|atendente|pessoa|agente|alguem|pessoa real)\b`),
		regexp.MustCompile(`\bparlare con (un |una )?(operatore|persona|umano|qualcuno|persona reale)\b`),
		regexp.MustCompile(`\b(spreken|praten) met (een )?(mens|medewerker|persoon|echt persoon)\b`),
		regexp.MustCompile(`\bmet (een )?(mens|medewerker|echt persoon) (spreken|praten)\b`),
		regexp.MustCompile(`(人間|担当者|オペレーター|人)と話`),
	}
}

/*
normalize lowercases, folds accents, turns punctuation into spaces and
collapses whitespace, so "Talk to a HUMAN!!" and "talk to a human"
read the same.
*/
func normalize(s string) string {
	s = accentFolder().Replace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

/*
accentFolder maps the accented letters of the matched languages to
their plain form.
*/
func accentFolder() *strings.Replacer {
	return strings.NewReplacer(
		"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a",
		"ç", "c", "è", "e", "é", "e", "ê", "e", "ë", "e",
		"ì", "i", "í", "i", "î", "i", "ï", "i", "ñ", "n",
		"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o",
		"ù", "u", "ú", "u", "û", "u", "ü", "u", "ß", "ss",
	)
}
