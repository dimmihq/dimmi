/*
|--------------------------------------------------------------------------
| Reading Answers
|--------------------------------------------------------------------------
|
| A customer answers a choice however they like: "2", "Billing",
| "bill", or "it's about billing please". The number wins, then a label
| typed out, then a unique start of a label of three letters or more,
| then a reply that holds exactly one label as whole words. Case and
| accents never matter. Anything else is not an answer to the choice.
|
| Variables in workflow text are filled from the contact, the company,
| the conversation and the run; a missing value takes the fallback the
| text gives, or nothing.
|
*/

package workflows

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
minPrefix is the shortest start of a label that counts as the label.
*/
const minPrefix = 3

/*
matchChoice returns the path the answer names.
*/
func matchChoice(paths []workflowspec.Path, answer string) (workflowspec.Path, bool) {
	text := str.FoldAccents(strings.TrimSpace(answer))
	if n, err := strconv.Atoi(strings.TrimSuffix(text, ".")); err == nil && n >= 1 && n <= len(paths) {
		return paths[n-1], true
	}

	for _, match := range []func(label string) bool{
		func(label string) bool { return text == label },
		func(label string) bool { return len([]rune(text)) >= minPrefix && strings.HasPrefix(label, text) },
		func(label string) bool { return containsWords(text, label) },
	} {
		if p, ok := only(paths, match); ok {
			return p, true
		}
	}

	return workflowspec.Path{}, false
}

/*
only returns the single path whose folded label matches, if exactly one
does.
*/
func only(paths []workflowspec.Path, match func(label string) bool) (workflowspec.Path, bool) {
	var found []workflowspec.Path
	for _, p := range paths {
		if match(str.FoldAccents(strings.TrimSpace(p.Label))) {
			found = append(found, p)
		}
	}

	if len(found) != 1 {
		return workflowspec.Path{}, false
	}

	return found[0], true
}

/*
containsWords reports whether the text holds the label's words, in
order, bounded by non-letters.
*/
func containsWords(text, label string) bool {
	words := strings.FieldsFunc(text, notWordRune)
	want := strings.FieldsFunc(label, notWordRune)

	if len(want) == 0 {
		return false
	}

	for i := 0; i+len(want) <= len(words); i++ {
		if strings.Join(words[i:i+len(want)], " ") == strings.Join(want, " ") {
			return true
		}
	}

	return false
}

/*
notWordRune reports whether a character separates words.
*/
func notWordRune(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

/*
loadInputText reads the text of the message a run receives.
*/
func (st *state) loadInputText(ctx context.Context) {
	msg, err := st.q.FindMessage(ctx, models.FindMessageParams{
		WorkspaceID: st.run.WorkspaceID, ConversationID: *st.run.ConversationID, ID: *st.input.MessageID,
	})
	if err == nil && msg.Body != nil {
		st.input.Text = *msg.Body
	}
}

/*
variablePattern finds a variable and its fallback in workflow text.
*/
func variablePattern() *regexp.Regexp {
	return regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_.]*)\s*(?:\|\s*fallback:\s*"([^"]*)"\s*)?\}\}`)
}

/*
render fills the variables of a text.
*/
func (st *state) render(ctx context.Context, text string) (string, error) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}

	s, err := st.subject(ctx)
	if err != nil {
		return "", err
	}

	return variablePattern().ReplaceAllStringFunc(text, func(found string) string {
		m := variablePattern().FindStringSubmatch(found)
		if v := st.variable(s, m[1]); v != "" {
			return v
		}

		return m[2]
	}), nil
}

/*
variable returns one variable's text, empty when it has no value.
*/
func (st *state) variable(s *subject, name string) string {
	switch name {
	case "contact.first_name":
		full, _ := s.Lookup("contact.name")
		first, _, _ := strings.Cut(fmt.Sprint(orEmpty(full)), " ")

		return first
	case "conversation.number":
		return strconv.FormatInt(st.conv.Number, 10)
	case "workspace.name":
		return ""
	}

	v, ok := s.Lookup(name)
	if !ok {
		return ""
	}

	return fmt.Sprint(v)
}

/*
orEmpty returns the value, or an empty text for none.
*/
func orEmpty(v any) any {
	if v == nil {
		return ""
	}

	return v
}
