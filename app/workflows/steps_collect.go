/*
|--------------------------------------------------------------------------
| Collecting Data
|--------------------------------------------------------------------------
|
| A collect step asks for one value, checks the answer and saves it.
| What a customer types is only ever a claim: an email they give is
| saved unverified and only when the contact has none, never merges two
| contacts and never proves who anyone is. A question whose answer is
| already known is skipped. A wrong answer is asked again, up to the
| attempts the step allows.
|
*/

package workflows

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The outcomes of a collect step.
*/
const (
	outcomeCollected     = "collected"
	outcomeAlreadyKnown  = "already_known"
	outcomeInvalidAnswer = "invalid_answer"
	defaultAttempts      = 2
)

/*
runCollect asks the question, unless the answer is already known.
*/
func runCollect(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	c := settings[workflowspec.CollectSettings](s)
	if workflowspec.On(c.SkipIfKnown, true) && st.known(ctx, c.Field) {
		return outcome{outcome: outcomeAlreadyKnown}, nil
	}

	text, err := st.render(ctx, c.Text)
	if err != nil {
		return outcome{}, err
	}

	if c.Type == "choice" || c.Type == "yes_no" {
		text += "\n\n" + numbered(choices(c))
	}

	sent, err := st.send(ctx, s, text, "")
	if err != nil || sent.outcome != outcomeSent {
		return sent, err
	}

	return outcome{wait: st.inputWait(c.Timeout), acted: true}, nil
}

/*
known reports whether the field already has a value.
*/
func (st *state) known(ctx context.Context, field string) bool {
	if key, ok := strings.CutPrefix(field, "run.vars."); ok {
		return st.vars[key] != nil
	}

	s, err := st.subject(ctx)
	if err != nil {
		return false
	}

	_, ok := s.Lookup(field)

	return ok
}

/*
resumeCollect checks the answer and saves it, asks again, or gives up.
*/
func resumeCollect(ctx context.Context, st *state, s workflowspec.Step, in Input) (outcome, error) {
	c := settings[workflowspec.CollectSettings](s)
	if in.Kind == InputTimer {
		return fixedOrNext(s, workflowspec.OutcomeTimeout), nil
	}

	value, ok := parseAnswer(c, in.Text)
	if ok {
		if err := st.store(ctx, c.Field, value); err != nil {
			return outcome{}, err
		}

		return outcome{outcome: outcomeCollected, acted: true, detail: map[string]any{"field": c.Field}}, nil
	}

	w := st.wait
	w.Attempts++

	limit := c.MaxAttempts
	if limit == 0 {
		limit = defaultAttempts
	}

	if w.Attempts >= limit {
		out := fixedOrNext(s, workflowspec.OutcomeInvalid)
		out.outcome = outcomeInvalidAnswer

		return out, nil
	}

	retry := c.InvalidText
	if retry == "" {
		retry = "Sorry, that doesn't look right. Could you try again?"
	}

	if _, err := st.send(ctx, s, retry, ":retry"+strconv.Itoa(w.Attempts)); err != nil {
		return outcome{}, err
	}

	return outcome{wait: &w, outcome: outcomeInvalidAnswer}, nil
}

/*
fixedOrNext takes the step's fixed path for the outcome, or goes on
along the step's own path when it has none.
*/
func fixedOrNext(s workflowspec.Step, fixed string) outcome {
	if p, ok := workflowspec.PathFor(s, fixed); ok {
		return outcome{path: p.ID, outcome: fixed}
	}

	return outcome{outcome: fixed}
}

/*
choices returns a choice question's options.
*/
func choices(c workflowspec.CollectSettings) []string {
	if c.Type == "yes_no" {
		return []string{"Yes", "No"}
	}

	return c.Options
}

/*
numbered lists options as numbered lines.
*/
func numbered(options []string) string {
	lines := make([]string, len(options))
	for i, o := range options {
		lines[i] = strconv.Itoa(i+1) + ". " + o
	}

	return strings.Join(lines, "\n")
}

/*
parseAnswer reads the answer as the step's type wants it.
*/
func parseAnswer(c workflowspec.CollectSettings, raw string) (any, bool) {
	text := strings.TrimSpace(str.CleanText(raw))
	if text == "" {
		return nil, false
	}

	switch c.Type {
	case "email":
		addr, err := mail.ParseAddress(text)
		return addr.Email, err == nil && strings.Contains(addr.Email, ".")
	case "phone":
		return phone(text)
	case "number":
		n, err := strconv.ParseFloat(strings.ReplaceAll(text, ",", ""), 64)
		return n, err == nil
	case "choice", "yes_no":
		return choose(choices(c), text)
	}

	return str.Limit(text, 1024), true
}

/*
phone reads a phone number as digits with an optional leading plus.
*/
func phone(text string) (any, bool) {
	var b strings.Builder
	for i, r := range text {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		case strings.ContainsRune(" -().", r):
		default:
			return nil, false
		}
	}

	digits := strings.TrimPrefix(b.String(), "+")

	return b.String(), len(digits) >= 7 && len(digits) <= 15
}

/*
choose matches an answer against options like buttons are matched.
*/
func choose(options []string, text string) (any, bool) {
	paths := make([]workflowspec.Path, len(options))
	for i, o := range options {
		paths[i] = workflowspec.Path{ID: o, Label: o}
	}

	p, ok := matchChoice(paths, text)

	return p.Label, ok
}

/*
store saves a collected value to its field.
*/
func (st *state) store(ctx context.Context, field string, value any) error {
	if key, ok := strings.CutPrefix(field, "run.vars."); ok {
		st.vars[key] = value
		return nil
	}

	contact, err := st.q.FindContact(ctx, models.FindContactParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ContactID,
	})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	params, ok := contactChange(contact, field, value)
	if !ok {
		return nil
	}

	params.Now = st.now()

	if _, err := st.q.UpdateContact(ctx, params); err != nil {
		return fmt.Errorf("save collected value: %w", err)
	}

	st.subj = nil

	return nil
}

/*
contactChange is the contact with the field set, and false when the
field must not change: an email the contact already has stays, and a
value custom attributes refuse is dropped.
*/
func contactChange(c models.Contact, field string, value any) (models.UpdateContactParams, bool) {
	p := models.UpdateContactParams{
		Kind: c.Kind, ExternalID: c.ExternalID, Email: c.Email, Name: c.Name, Phone: c.Phone,
		CustomAttributes: c.CustomAttributes, TrustedAttributeKeys: c.TrustedAttributeKeys, BlockedAt: c.BlockedAt,
		Notes: c.Notes, CompanyID: c.CompanyID, EmailVerified: c.EmailVerified, Now: c.UpdatedAt,
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
	text := fmt.Sprint(value)

	switch field {
	case "contact.email":
		if c.Email != nil {
			return p, false
		}

		p.Email, p.EmailVerified = &text, false
	case "contact.name":
		p.Name = &text
	case "contact.phone":
		p.Phone = &text
	default:
		key := strings.TrimPrefix(field, "contact.custom.")
		merged, _, err := attributes.Merge(c.CustomAttributes, map[string]any{key: value})
		p.CustomAttributes = merged

		return p, err == nil
	}

	return p, true
}
