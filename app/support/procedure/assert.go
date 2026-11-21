/*
|--------------------------------------------------------------------------
| Assertions
|--------------------------------------------------------------------------
|
| A saved test case states what must hold after the simulated
| conversation: the procedure matched, an action was or was not called,
| one before another, a branch went one way, the conversation was
| handed over, the reply said or left out something. Code checks each
| one against what the simulation observed; only a rubric, written in
| plain words, is judged by a model, and only when one is given.
|
*/

package procedure

import (
	"encoding/json"
	"slices"
	"strings"
)

/*
The assertion types.
*/
const (
	AssertProcedureMatched = "procedure_matched"
	AssertActionCalled     = "action_called"
	AssertActionNotCalled  = "action_not_called"
	AssertActionOrder      = "action_order"
	AssertBranchTaken      = "branch_taken"
	AssertHandedOff        = "handed_off"
	AssertReplyContains    = "reply_contains"
	AssertReplyNotContains = "reply_not_contains"
	AssertLLMRubric        = "llm_rubric"
	maxAssertions          = 30
	maxAssertionTextRunes  = 500
)

/*
Assertion is one thing a test case expects. Only the fields its type
reads are set: Procedure and Matched for a match, Action and Args for a
call, Action and Before for an order, Step and Branch for a branch,
Team and Priority for a handover, Text for a reply, Rubric for a model's
judgment. An action is named by public id or tool name.
*/
type Assertion struct {
	Type      string         `json:"type"`
	Procedure string         `json:"procedure_id,omitempty"`
	Matched   *bool          `json:"matched,omitempty"`
	Action    string         `json:"action,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Before    string         `json:"before,omitempty"`
	Step      string         `json:"step_id,omitempty"`
	Branch    string         `json:"branch,omitempty"`
	HandedOff *bool          `json:"handed_off,omitempty"`
	Team      string         `json:"team_id,omitempty"`
	Priority  string         `json:"priority,omitempty"`
	Text      string         `json:"text,omitempty"`
	Rubric    string         `json:"rubric,omitempty"`
}

/*
ObservedCall is one action the simulation called.
*/
type ObservedCall struct {
	ActionID  string         `json:"action_id"`
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args,omitempty"`
	Succeeded bool           `json:"succeeded"`
	Blocked   bool           `json:"blocked,omitempty"`
}

/*
Observed is what a simulated conversation did, across its turns.
*/
type Observed struct {
	Procedures []string          `json:"procedures"`
	Calls      []ObservedCall    `json:"calls"`
	Branches   map[string]string `json:"branches"`
	HandedOff  bool              `json:"handed_off"`
	Team       string            `json:"team_id,omitempty"`
	Priority   string            `json:"priority,omitempty"`
	Replies    []string          `json:"replies"`
}

/*
Judge decides a rubric over a transcript, with its reason.
*/
type Judge func(rubric, transcript string) (bool, string, error)

/*
AssertionResult is how one assertion came out.
*/
type AssertionResult struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	Skipped bool   `json:"skipped,omitempty"`
}

/*
assertionChecks say, by assertion type, what is wrong with one, empty
when nothing is.
*/
func assertionChecks() map[string]func(Assertion) string {
	words := func(text, what string) string {
		if strings.TrimSpace(text) == "" || len([]rune(text)) > maxAssertionTextRunes {
			return "Write the " + what + " in 1 to 500 characters."
		}
		return ""
	}
	action := func(a Assertion) string {
		return map[bool]string{true: "Name the action.", false: ""}[a.Action == ""]
	}
	return map[string]func(Assertion) string{
		AssertProcedureMatched: func(Assertion) string { return "" },
		AssertHandedOff:        func(Assertion) string { return "" },
		AssertActionCalled:     action,
		AssertActionNotCalled:  action,
		AssertActionOrder: func(a Assertion) string {
			return map[bool]string{true: "Name both actions.", false: ""}[a.Action == "" || a.Before == ""]
		},
		AssertBranchTaken: func(a Assertion) string {
			bad := a.Step == "" || (a.Branch != "then" && a.Branch != "else")
			return map[bool]string{true: "Name the step and the side it takes.", false: ""}[bad]
		},
		AssertReplyContains:    func(a Assertion) string { return words(a.Text, "text") },
		AssertReplyNotContains: func(a Assertion) string { return words(a.Text, "text") },
		AssertLLMRubric:        func(a Assertion) string { return words(a.Rubric, "rubric") },
	}
}

/*
CheckAssertions lists what is wrong with a test case's assertions.
*/
func CheckAssertions(as []Assertion) []Issue {
	var out []Issue
	if len(as) > maxAssertions {
		out = append(out, Issue{Code: "limit_exceeded", Field: "assertions", Message: "Write at most 30 assertions."})
	}

	checks := assertionChecks()
	for _, a := range as {
		msg := "This assertion type does not exist."
		if check, ok := checks[a.Type]; ok {
			msg = check(a)
		}
		if msg != "" {
			out = append(out, Issue{Code: "invalid_setting", Field: "assertions", Message: msg})
		}
	}

	return out
}

/*
Assert checks every assertion against what was observed. A rubric with
no judge is skipped, never failed.
*/
func Assert(as []Assertion, o Observed, transcript string, judge Judge) []AssertionResult {
	out := make([]AssertionResult, 0, len(as))
	for _, a := range as {
		r := assertOne(a, o, transcript, judge)
		r.Type = a.Type
		out = append(out, r)
	}

	return out
}

/*
Passed reports whether every result passed or was skipped.
*/
func Passed(rs []AssertionResult) bool {
	for _, r := range rs {
		if !r.Passed && !r.Skipped {
			return false
		}
	}

	return true
}

/*
assertOne checks one assertion.
*/
func assertOne(a Assertion, o Observed, transcript string, judge Judge) AssertionResult {
	switch a.Type {
	case AssertProcedureMatched:
		return matched(a, o)
	case AssertActionCalled:
		return verdict(called(o, a.Action, a.Args), a.Action+" was called", a.Action+" was not called")
	case AssertActionNotCalled:
		return verdict(!called(o, a.Action, nil), a.Action+" was not called", a.Action+" was called")
	case AssertActionOrder:
		first, second := firstCall(o, a.Action), firstCall(o, a.Before)
		ok := first >= 0 && second >= 0 && first < second && o.Calls[first].Succeeded
		return verdict(ok, a.Action+" succeeded before "+a.Before, a.Action+" did not succeed before "+a.Before)
	case AssertBranchTaken:
		got := o.Branches[a.Step]
		return verdict(got == a.Branch, "step "+a.Step+" took "+a.Branch,
			"step "+a.Step+" took "+orNone(got)+", not "+a.Branch)
	case AssertHandedOff:
		return handedOff(a, o)
	case AssertReplyContains, AssertReplyNotContains:
		return replied(a, o)
	case AssertLLMRubric:
		return rubric(a, transcript, judge)
	}

	return AssertionResult{Message: "unknown assertion"}
}

/*
matched checks the procedure matched, or did not.
*/
func matched(a Assertion, o Observed) AssertionResult {
	want := a.Matched == nil || *a.Matched
	got := len(o.Procedures) > 0
	if a.Procedure != "" {
		got = slices.Contains(o.Procedures, a.Procedure)
	}

	return verdict(got == want, "the procedure matched as expected", "the procedure did not match as expected")
}

/*
replied checks a reply said, or left out, some text.
*/
func replied(a Assertion, o Observed) AssertionResult {
	has := strings.Contains(strings.ToLower(strings.Join(o.Replies, "\n")), strings.ToLower(a.Text))
	if a.Type == AssertReplyNotContains {
		return verdict(!has, "no reply says \""+a.Text+"\"", "a reply says \""+a.Text+"\"")
	}

	return verdict(has, "the reply says \""+a.Text+"\"", "no reply says \""+a.Text+"\"")
}

/*
rubric asks the judge, skipping when there is none or it fails.
*/
func rubric(a Assertion, transcript string, judge Judge) AssertionResult {
	if judge == nil {
		return AssertionResult{Skipped: true, Passed: true, Message: "No judge model is set up; skipped."}
	}

	ok, why, err := judge(a.Rubric, transcript)
	if err != nil {
		return AssertionResult{Skipped: true, Passed: true, Message: "The judge failed: " + err.Error()}
	}

	return AssertionResult{Passed: ok, Message: why}
}

/*
handedOff checks a handover, and its team and priority when named.
*/
func handedOff(a Assertion, o Observed) AssertionResult {
	want := a.HandedOff == nil || *a.HandedOff
	if o.HandedOff != want {
		return verdict(false, "", "the conversation was handed over: "+boolWord(o.HandedOff))
	}

	if want && a.Team != "" && a.Team != o.Team {
		return AssertionResult{Message: "handed to " + orNone(o.Team) + ", not " + a.Team}
	}

	if want && a.Priority != "" && a.Priority != o.Priority {
		return AssertionResult{Message: "handed over at " + orNone(o.Priority) + " priority, not " + a.Priority}
	}

	return AssertionResult{Passed: true, Message: "handed over: " + boolWord(want)}
}

/*
called reports whether an action was called, with every given argument.
*/
func called(o Observed, action string, args map[string]any) bool {
	for _, c := range o.Calls {
		if c.Blocked || (c.ActionID != action && c.Tool != action) {
			continue
		}
		match := true
		for k, v := range args {
			match = match && equal(c.Args[k], v)
		}
		if match {
			return true
		}
	}

	return false
}

/*
firstCall is the index of an action's first call that was not blocked.
*/
func firstCall(o Observed, action string) int {
	return slices.IndexFunc(o.Calls, func(c ObservedCall) bool {
		return !c.Blocked && (c.ActionID == action || c.Tool == action)
	})
}

/*
verdict is a result with the message for its outcome.
*/
func verdict(ok bool, pass, fail string) AssertionResult {
	return AssertionResult{Passed: ok, Message: map[bool]string{true: pass, false: fail}[ok]}
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}

	return s
}

func boolWord(b bool) string {
	return map[bool]string{true: "yes", false: "no"}[b]
}

/*
ParseAssertions reads stored assertions.
*/
func ParseAssertions(raw json.RawMessage) ([]Assertion, error) {
	out := []Assertion{}
	if len(raw) == 0 {
		return out, nil
	}

	err := json.Unmarshal(raw, &out)

	return out, err
}
