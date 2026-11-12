/*
|--------------------------------------------------------------------------
| Checking Ticket And SLA Steps
|--------------------------------------------------------------------------
|
| Ticket steps name the type, state, tracker or team they act on, as
| picked in the builder, and field values by key. Values and titles may
| hold variables, which must be ones the engine can fill. Applying an
| SLA names its policy.
|
*/

package workflowspec

import (
	"strings"
	"unicode/utf8"
)

/*
Limits of ticket step settings.
*/
const (
	maxTicketValues = 30
	maxTicketValue  = 2000
	maxTicketTitle  = 200
)

/*
ticketChecks returns the checks of the ticket and SLA steps.
*/
func (c *checker) ticketChecks() map[string]func(Step) {
	return map[string]func(Step){
		KindCreateTicket: c.checkCreateTicket, KindSetTicketState: c.checkSetTicketState,
		KindSetTicketFields: c.checkSetTicketFields, KindEscalate: c.checkEscalate,
		KindAddToTracker: c.checkAddToTracker, KindApplySLA: c.checkApplySLA,
	}
}

/*
checkCreateTicket checks the type, title and values of a new ticket.
*/
func (c *checker) checkCreateTicket(s Step) {
	t, ok := decode[CreateTicketSettings](c, s)
	if !ok {
		return
	}

	c.required(s, "type_id", t.TypeID, "Pick a ticket type.")
	c.title(s, t.Title)
	c.ticketValues(s, t.Fields)
}

/*
checkSetTicketState checks the state to move to.
*/
func (c *checker) checkSetTicketState(s Step) {
	if t, ok := decode[SetTicketStateSettings](c, s); ok {
		c.required(s, "state_id", t.StateID, "Pick a state.")
	}
}

/*
checkSetTicketFields checks the values to set.
*/
func (c *checker) checkSetTicketFields(s Step) {
	t, ok := decode[SetTicketFieldsSettings](c, s)
	if !ok {
		return
	}

	if len(t.Fields) == 0 {
		c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "fields", Message: "Set at least one field."})
	}

	c.ticketValues(s, t.Fields)
}

/*
checkEscalate checks the back-office type and title.
*/
func (c *checker) checkEscalate(s Step) {
	t, ok := decode[EscalateSettings](c, s)
	if !ok {
		return
	}

	c.required(s, "type_id", t.TypeID, "Pick a back-office type.")
	c.required(s, "title", t.Title, "Write the title.")
	c.title(s, t.Title)
	c.ticketValues(s, t.Fields)
}

/*
checkAddToTracker checks the tracker.
*/
func (c *checker) checkAddToTracker(s Step) {
	if t, ok := decode[AddToTrackerSettings](c, s); ok {
		c.required(s, "tracker_id", t.TrackerID, "Pick a tracker.")
	}
}

/*
checkApplySLA checks the policy.
*/
func (c *checker) checkApplySLA(s Step) {
	if t, ok := decode[ApplySLASettings](c, s); ok {
		c.required(s, "sla_policy_id", t.SLAPolicyID, "Pick an SLA policy.")
	}
}

/*
title checks a ticket title's length and variables.
*/
func (c *checker) title(s Step, value string) {
	if utf8.RuneCountInString(value) > maxTicketTitle {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "title", Message: "Titles are 200 characters."})
	}

	c.variables(s, "title", value)
}

/*
ticketValues checks field values: a key each, said once, and values
with variables the engine can fill.
*/
func (c *checker) ticketValues(s Step, values []TicketValue) {
	if len(values) > maxTicketValues {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Field: "fields", Message: "Set 30 fields at most."})
	}

	seen := map[string]bool{}
	for _, v := range values {
		if !keyPattern().MatchString(v.Key) || seen[v.Key] {
			c.fail(Issue{
				Code: CodeInvalidSetting, StepID: s.ID, Field: "fields",
				Message: "Pick each field once.",
			})
		}

		seen[v.Key] = true
		if strings.TrimSpace(v.Value) == "" || utf8.RuneCountInString(v.Value) > maxTicketValue {
			c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Field: "fields", Message: "Give each field a value."})
		}

		c.variables(s, "fields", v.Value)
	}
}
