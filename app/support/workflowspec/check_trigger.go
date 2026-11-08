/*
|--------------------------------------------------------------------------
| Trigger Settings
|--------------------------------------------------------------------------
|
| Some triggers take settings of their own. The quiet triggers need to
| know how long the silence lasts and whose reply starts it, and may
| count only office hours; a status change may be narrowed to the
| statuses that matter.
|
*/

package workflowspec

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"
)

/*
QuietSettings are the settings of the quiet triggers. After is how long
the silence lasts; AfterReplyFrom says whose reply starts a customer's
silence (member, dimmi or both, the default); Count business counts
only the open hours of the schedule, the default one when none is
named.
*/
type QuietSettings struct {
	After          string `json:"after"`
	AfterReplyFrom string `json:"after_reply_from,omitempty"`
	Count          string `json:"count,omitempty"`
	ScheduleID     string `json:"schedule_id,omitempty"`
}

/*
ReadQuiet reads a quiet trigger's settings.
*/
func ReadQuiet(raw json.RawMessage) QuietSettings {
	var q QuietSettings
	_ = json.Unmarshal(raw, &q)

	return q
}

/*
StatusSettings narrow a status or assignment trigger to where the
conversation went.
*/
type StatusSettings struct {
	To []string `json:"to,omitempty"`
}

/*
checkTriggerSettings checks the settings of the triggers that take
some.
*/
func (c *checker) checkTriggerSettings() {
	raw := c.spec.TriggerSettings
	switch c.spec.Trigger {
	case TriggerCustomerQuiet:
		c.checkQuiet(raw, 30*24*time.Hour)
	case TriggerTeammateQuiet:
		c.checkQuiet(raw, 7*24*time.Hour)
	case TriggerStatusChanged:
		c.checkStatusTo(raw)
	}
}

/*
checkQuiet checks a quiet trigger's silence and counting.
*/
func (c *checker) checkQuiet(raw json.RawMessage, longest time.Duration) {
	var q QuietSettings
	if !c.strict(raw, &q) {
		return
	}

	d, err := ParseDuration(q.After)
	if err != nil || d < time.Minute || d > longest {
		c.fail(Issue{
			Code: CodeInvalidDuration, Field: "trigger_settings.after",
			Message: "Say how long the silence lasts, from 1 minute to " + longestText(longest) + ".",
		})
	}

	if q.AfterReplyFrom != "" && !slices.Contains([]string{"member", "dimmi", "both"}, q.AfterReplyFrom) {
		c.fail(Issue{
			Code: CodeInvalidSetting, Field: "trigger_settings.after_reply_from", Message: "Use member, dimmi or both.",
		})
	}

	if q.Count != "" && q.Count != "calendar" && q.Count != "business" {
		c.fail(Issue{Code: CodeInvalidSetting, Field: "trigger_settings.count", Message: "Use calendar or business."})
	}
}

/*
checkStatusTo checks the statuses a status trigger narrows to.
*/
func (c *checker) checkStatusTo(raw json.RawMessage) {
	var s StatusSettings
	if !c.strict(raw, &s) {
		return
	}

	for _, to := range s.To {
		if !slices.Contains([]string{"open", "snoozed", "closed", "any"}, to) {
			c.fail(Issue{
				Code: CodeInvalidSetting, Field: "trigger_settings.to", Message: "Use open, snoozed or closed.",
			})
		}
	}
}

/*
strict decodes trigger settings, refusing unknown keys.
*/
func (c *checker) strict(raw json.RawMessage, out any) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		c.fail(Issue{
			Code: CodeInvalidSetting, Field: "trigger_settings",
			Message: "These settings cannot be read: " + err.Error(),
		})

		return false
	}

	return true
}

/*
longestText says a longest silence in words.
*/
func longestText(d time.Duration) string {
	if d == 7*24*time.Hour {
		return "7 days"
	}

	return "30 days"
}
