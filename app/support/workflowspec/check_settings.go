/*
|--------------------------------------------------------------------------
| Checking Step Settings
|--------------------------------------------------------------------------
|
| Each kind's settings are read strictly, then checked for what the
| engine needs to run the step: the text it sends, the field it saves
| to, the ids it acts on, durations in range.
|
*/

package workflowspec

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

/*
decode reads a step's settings strictly into T.
*/
func decode[T any](c *checker, s Step) (T, bool) {
	var out T
	raw := s.Settings
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&out); err != nil {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Message: "These settings cannot be read: " + err.Error()})
		return out, false
	}

	return out, true
}

/*
checkSettings checks a step's settings by kind.
*/
func (c *checker) checkSettings(s Step) {
	checks := map[string]func(Step){
		KindMessage: c.checkMessage, KindButtons: c.checkButtons, KindCollect: c.checkCollect,
		KindBranch: c.checkBranch, KindWait: c.checkWait, KindAssign: c.checkAssign,
		KindUnassign: c.checkUnassign, KindTag: c.checkTags, KindUntag: c.checkTags,
		KindPriority: c.checkPriority, KindNote: c.checkNote, KindUpdateContact: c.checkUpdateContact,
		KindSnooze: c.checkSnooze, KindClose: c.checkClose, KindDimmi: c.checkDimmi,
	}

	if check, ok := checks[s.Kind]; ok {
		check(s)
	}
}

/*
text checks a required text setting's length and variables.
*/
func (c *checker) text(s Step, value string, limit int) {
	const field = "text"
	at := Issue{StepID: s.ID, Field: field}
	if strings.TrimSpace(value) == "" {
		at.Code, at.Message = CodeMissingSetting, "Write the text."
		c.fail(at)

		return
	}

	if utf8.RuneCountInString(value) > limit {
		at.Code, at.Message = CodeLimitExceeded, "This text is too long."
		c.fail(at)
	}

	c.variables(s, field, value)
}

/*
variables refuses variables the engine cannot fill.
*/
func (c *checker) variables(s Step, field, value string) {
	for _, name := range Variables(value) {
		if !KnownVariable(name) {
			c.fail(Issue{
				Code: CodeUnknownVariable, StepID: s.ID, Field: field,
				Message: "No variable is called " + name + ".",
			})
		}
	}
}

/*
span is the range a duration setting must fall in.
*/
type span struct {
	lowest, highest time.Duration
}

/*
duration checks an optional duration setting's range.
*/
func (c *checker) duration(s Step, field, value string, within span) {
	if value == "" {
		return
	}

	d, err := ParseDuration(value)
	if err != nil || d < within.lowest || d > within.highest {
		c.fail(Issue{Code: CodeInvalidDuration, StepID: s.ID, Field: field, Message: "This duration is out of range."})
	}
}

/*
required refuses an empty setting, saying what to fill in.
*/
func (c *checker) required(s Step, field, value, message string) {
	if strings.TrimSpace(value) == "" {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: field, Message: message})
	}
}

/*
oneOf refuses a setting outside its allowed values.
*/
func (c *checker) oneOf(s Step, field, value string, allowed ...string) {
	if !slices.Contains(allowed, value) {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID, Field: field,
			Message: "Use one of " + strings.Join(allowed, ", ") + ".",
		})
	}
}

/*
checkMessage checks a message step.
*/
func (c *checker) checkMessage(s Step) {
	m, ok := decode[MessageSettings](c, s)
	if !ok {
		return
	}

	c.text(s, m.Text, maxMessage)
	c.duration(s, "delay", m.Delay, span{0, 30 * time.Second})
}

/*
checkButtons checks a buttons step and its labels.
*/
func (c *checker) checkButtons(s Step) {
	b, ok := decode[ButtonsSettings](c, s)
	if !ok {
		return
	}

	c.text(s, b.Text, maxMessage)
	c.duration(s, "timeout", b.Timeout, span{time.Minute, maxDuration})

	for _, p := range userPaths(s) {
		if n := utf8.RuneCountInString(strings.TrimSpace(p.Label)); n == 0 || n > maxLabel {
			c.fail(Issue{
				Code: CodeMissingSetting, StepID: s.ID, PathID: p.ID,
				Message: "Label each button, 40 characters at most.",
			})
		}
	}

	if _, typed := PathFor(s, OutcomeTyped); typed && !On(b.AllowTyping, true) {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Message: "Allow typing to have a typed path."})
	}
}

/*
checkCollect checks a collect step's target and answer type.
*/
func (c *checker) checkCollect(s Step) {
	col, ok := decode[CollectSettings](c, s)
	if !ok {
		return
	}

	c.text(s, col.Text, maxMessage)
	c.duration(s, "timeout", col.Timeout, span{time.Minute, maxDuration})
	c.oneOf(s, "type", col.Type, "text", "email", "phone", "number", "choice", "yes_no")

	if !collectTarget(col.Field) {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID, Field: "field",
			Message: "Save to contact.email, contact.name, contact.phone, contact.custom.<key> or run.vars.<key>.",
		})
	}

	if col.Type == "choice" && (len(col.Options) < 2 || len(col.Options) > 10) {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "options", Message: "Offer 2 to 10 choices."})
	}

	if col.MaxAttempts < 0 || col.MaxAttempts > 3 {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: "max_attempts", Message: "Allow 1 to 3 attempts."})
	}
}

/*
collectTarget reports whether a collect step may save to the field.
*/
func collectTarget(field string) bool {
	switch field {
	case "contact.email", "contact.name", "contact.phone":
		return true
	}

	for _, prefix := range []string{"contact.custom.", "run.vars."} {
		if rest, ok := strings.CutPrefix(field, prefix); ok && keyPattern().MatchString(rest) {
			return true
		}
	}

	return false
}
