/*
|--------------------------------------------------------------------------
| Checking Logic and Action Steps
|--------------------------------------------------------------------------
|
| Branches need a condition on every path but the last, waits need one
| clear end, and actions need the ids they act on. Contact updates may
| never touch the email or external id: those identify a person, and a
| workflow must never be able to merge or impersonate anyone.
|
*/

package workflowspec

import (
	"regexp"
	"strings"
	"time"
)

/*
keyPattern is the shape of custom attribute and variable keys.
*/
func keyPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
}

/*
checkBranch checks each path's label and condition.
*/
func (c *checker) checkBranch(s Step) {
	if len(s.Settings) > 0 && string(s.Settings) != "{}" && string(s.Settings) != "null" {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID,
			Message: "A branch has no settings: conditions live on its paths.",
		})
	}

	for _, p := range userPaths(s) {
		if strings.TrimSpace(p.Label) == "" {
			c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, PathID: p.ID, Message: "Name this path."})
		}

		c.checkCondition(p.Condition, Issue{StepID: s.ID, PathID: p.ID, Field: "condition"})
	}
}

/*
checkWait checks that a wait has exactly one end.
*/
func (c *checker) checkWait(s Step) {
	w, ok := decode[WaitSettings](c, s)
	if !ok {
		return
	}

	if (w.For == "") == (w.Until == nil) {
		c.fail(Issue{
			Code: CodeMissingSetting, StepID: s.ID, Field: "for",
			Message: "Wait for a duration or until a moment.",
		})
	}

	c.duration(s, "for", w.For, span{time.Minute, maxDuration})
	c.until(s, w.Until)

	if w.Count == "business" && w.For == "" {
		c.fail(Issue{
			Code: CodeInvalidSetting, StepID: s.ID, Field: "count",
			Message: "Count office hours only with a wait for a duration.",
		})
	}

	if w.Count != "" {
		c.oneOf(s, "count", w.Count, "calendar", "business")
	}

	for _, source := range w.InterruptOn {
		c.oneOf(s, "interrupt_on", source, "customer_message", "teammate_reply")
	}
}

/*
until checks a moment to wait or snooze until.
*/
func (c *checker) until(s Step, u *WaitUntil) {
	if u == nil {
		return
	}

	c.oneOf(s, "until.kind", u.Kind, "office_hours_open", "time_of_day")

	if u.Kind != "time_of_day" {
		return
	}

	if _, err := time.Parse("15:04", u.Time); err != nil {
		c.fail(Issue{Code: CodeInvalidSetting, StepID: s.ID, Field: "until.time", Message: "Use a time like 09:00."})
	}
}

/*
checkAssign checks an assignment's mode and targets.
*/
func (c *checker) checkAssign(s Step) {
	a, ok := decode[AssignSettings](c, s)
	if !ok {
		return
	}

	c.oneOf(s, "mode", a.Mode, "member", "team", "team_round_robin", "team_balanced")

	if a.Mode == "member" {
		c.required(s, "member_id", a.MemberID, "Pick a teammate.")
	} else {
		c.required(s, "team_id", a.TeamID, "Pick a team.")
	}

	if a.Fallback != "" {
		c.oneOf(s, "fallback", a.Fallback, "team_only", "leave")
	}
}

/*
checkUnassign checks what to unassign.
*/
func (c *checker) checkUnassign(s Step) {
	if u, ok := decode[UnassignSettings](c, s); ok {
		c.oneOf(s, "what", u.What, "member", "team", "both")
	}
}

/*
checkTags checks a tag list.
*/
func (c *checker) checkTags(s Step) {
	t, ok := decode[TagSettings](c, s)
	if ok && (len(t.TagIDs) == 0 || len(t.TagIDs) > maxTags) {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "tag_ids", Message: "Pick 1 to 20 tags."})
	}
}

/*
checkPriority checks a priority level.
*/
func (c *checker) checkPriority(s Step) {
	if p, ok := decode[PrioritySettings](c, s); ok {
		c.oneOf(s, "value", p.Value, "none", "low", "medium", "high", "urgent")
	}
}

/*
checkNote checks a note's text.
*/
func (c *checker) checkNote(s Step) {
	if n, ok := decode[NoteSettings](c, s); ok {
		c.text(s, n.Text, maxNote)
	}
}

/*
checkUpdateContact checks contact changes.
*/
func (c *checker) checkUpdateContact(s Step) {
	u, ok := decode[UpdateContactSettings](c, s)
	if !ok {
		return
	}

	if len(u.Set) == 0 || len(u.Set) > maxSetFields {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "set", Message: "Set 1 to 10 fields."})
	}

	for _, change := range u.Set {
		key, custom := strings.CutPrefix(change.Field, "contact.custom.")
		named := change.Field == "contact.name" || change.Field == "contact.phone"
		if !named && (!custom || !keyPattern().MatchString(key)) {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "set",
				Message: "Set contact.name, contact.phone or contact.custom.<key>; never the email or external id.",
			})
		}

		c.variables(s, "set", change.Value)
	}
}

/*
checkSnooze checks a snooze's end.
*/
func (c *checker) checkSnooze(s Step) {
	sn, ok := decode[SnoozeSettings](c, s)
	if !ok {
		return
	}

	if (sn.For == "") == (sn.Until == nil) {
		c.fail(Issue{
			Code: CodeMissingSetting, StepID: s.ID, Field: "for",
			Message: "Snooze for a duration or until a moment.",
		})
	}

	c.duration(s, "for", sn.For, span{time.Minute, maxDuration})
	c.until(s, sn.Until)
}

/*
checkClose checks a close step: its settings must read as a close's.
*/
func (c *checker) checkClose(s Step) {
	decode[CloseSettings](c, s)
}
