/*
|--------------------------------------------------------------------------
| Checking Definitions
|--------------------------------------------------------------------------
|
| A definition is checked as a whole before it may go live: the trigger
| exists and the steps suit it, every step's settings are complete and
| in range, ids are unique, conditions read fields the trigger has, and
| the tree stays within the limits that keep runs fast and readable.
| References to teams, members and tags are only collected here; the
| caller checks they exist, because that needs the database.
|
*/

package workflowspec

import (
	"errors"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
Limits of a definition.
*/
const (
	MaxSteps      = 150
	MaxPathDepth  = 12
	maxMessage    = 4000
	maxNote       = 10000
	maxLabel      = 40
	maxDuration   = 30 * 24 * time.Hour
	maxTags       = 20
	maxSetFields  = 10
	maxOpenerHead = 5
)

/*
checker collects issues while walking a definition.
*/
type checker struct {
	spec    Spec
	trigger Trigger
	report  *Report
	ids     map[string]bool
	steps   int
}

/*
Check validates a definition and returns every error and warning.
*/
func Check(s Spec) Report {
	report := Report{Errors: []Issue{}, Warnings: []Issue{}}
	c := &checker{spec: s, report: &report, ids: map[string]bool{}}

	if s.Schema != SchemaVersion {
		c.fail(Issue{Code: CodeSchema, Field: "schema", Message: "Use definition schema 1."})
	}

	t, ok := FindTrigger(s.Trigger)
	if !ok {
		c.fail(Issue{Code: CodeSchema, Field: "trigger", Message: "No trigger has this name."})
		return report
	}

	c.trigger = t
	c.checkHead()
	c.checkSteps(s.Steps, 1)

	if c.steps > MaxSteps {
		c.fail(Issue{Code: CodeLimitExceeded, Field: "steps", Message: "Use 150 steps at most."})
	}

	if t.Key == TriggerOpener {
		c.checkOpener()
	}

	return report
}

/*
fail records an error.
*/
func (c *checker) fail(i Issue) {
	c.report.Errors = append(c.report.Errors, i)
}

/*
warn records a warning.
*/
func (c *checker) warn(i Issue) {
	c.report.Warnings = append(c.report.Warnings, i)
}

/*
checkHead checks everything above the steps: channels, audience,
schedule and re-entry.
*/
func (c *checker) checkHead() {
	s, t := c.spec, c.trigger
	for _, ch := range s.Channels {
		if len(t.Channels) > 0 && !slices.Contains(t.Channels, ch) {
			c.fail(Issue{
				Code: CodeChannelUnsupported, Field: "channels",
				Message: "This trigger cannot run on " + ch + ".",
			})
		}
	}

	if t.Key == TriggerReusable && (len(s.Audience) > 0 || s.Schedule.Mode != "" && s.Schedule.Mode != "any") {
		c.warn(Issue{
			Code: WarnIgnoredOnReusable, Field: "audience",
			Message: "Reusable workflows run for whoever calls them: audience and schedule are ignored.",
		})
	}

	c.checkAudience()

	switch s.Schedule.Mode {
	case "", "any", "open", "closed":
	default:
		c.fail(Issue{Code: CodeInvalidSetting, Field: "schedule.mode", Message: "Use any, open or closed."})
	}

	c.checkReentry()
}

/*
checkReentry checks the re-entry rule against what the trigger allows.
*/
func (c *checker) checkReentry() {
	r := c.spec.Reentry
	if r.Mode == "" {
		return
	}

	if !slices.Contains(c.trigger.Reentry, r.Mode) {
		c.fail(Issue{
			Code: CodeInvalidSetting, Field: "reentry.mode",
			Message: "This trigger cannot use this re-entry rule.",
		})
		return
	}

	if r.Mode != ReentryCooldown {
		return
	}

	d, err := ParseDuration(r.Cooldown)
	if err != nil || d < time.Minute || d > 90*24*time.Hour {
		c.fail(Issue{Code: CodeInvalidDuration, Field: "reentry.cooldown", Message: "Use 1 minute to 90 days."})
	}
}

/*
checkAudience compiles the audience and refuses fields the trigger does
not have.
*/
func (c *checker) checkAudience() {
	a, err := audience.Parse(c.spec.Audience)
	if err != nil {
		c.conditionProblems(err, Issue{Field: "audience"})
		return
	}

	surface := audience.WorkflowAudience()
	if c.trigger.Key == TriggerOpener {
		surface = audience.WorkflowOpener()
	}

	if _, err := audience.Compile(a, Catalog(), surface); err != nil {
		c.conditionProblems(err, Issue{Field: "audience"})
		return
	}

	c.checkFields(a.Rules, Issue{Field: "audience"}, scopeAudience)
}

/*
checkCondition compiles a branch condition.
*/
func (c *checker) checkCondition(raw []byte, at Issue) {
	r, err := audience.ParseRule(raw)
	if err != nil {
		c.conditionProblems(err, at)
		return
	}

	if r == nil {
		at.Code, at.Message = CodeMissingSetting, "Add a condition to this path."
		c.fail(at)

		return
	}

	if _, err := audience.CompileRule(r, Catalog(), audience.WorkflowBranch()); err != nil {
		c.conditionProblems(err, at)
		return
	}

	c.checkFields(r, at, scopeBranch)
}

/*
checkFields refuses fields the trigger cannot fill and warns about
claimed values, which a visitor could set to reach a route.
*/
func (c *checker) checkFields(r *audience.Rule, at Issue, scope ruleScope) {
	for _, leaf := range leaves(r) {
		f, _ := Catalog().Lookup(leaf.Field)
		issue := at
		issue.Field = at.Field + ":" + leaf.Field

		if !fieldAllowed(f, c.trigger, scope) {
			issue.Code, issue.Message = CodeFieldUnavailableTrigger, "This trigger has no "+leaf.Field+"."
			c.fail(issue)

			continue
		}

		if f.Trust == audience.Claimed && scope == scopeAudience {
			issue.Code = WarnClaimedField
			issue.Message = "A visitor can set " + leaf.Field + " themselves; do not rely on it to grant anything."
			c.warn(issue)
		}
	}
}

/*
conditionProblems turns rule problems into definition errors.
*/
func (c *checker) conditionProblems(err error, at Issue) {
	var pe *audience.ProblemsError
	if !errors.As(err, &pe) {
		at.Code, at.Message = CodeInvalidCondition, err.Error()
		c.fail(at)

		return
	}

	for _, p := range pe.Problems {
		issue := at
		issue.Code, issue.Message = CodeInvalidCondition, p.Message
		issue.Field = at.Field + ":" + p.Path
		if p.Code == audience.CodeFieldUnavailable {
			issue.Code = CodeFieldUnavailableTrigger
		}

		c.fail(issue)
	}
}
