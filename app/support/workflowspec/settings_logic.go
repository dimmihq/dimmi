/*
|--------------------------------------------------------------------------
| Logic Step Settings
|--------------------------------------------------------------------------
|
| The typed settings of more step kinds; see the step settings file for
| how ids and optional switches are written.
|
*/

package workflowspec

/*
ClassifySettings lets dimmi pick one of the paths for the customer's
words: each path is a category with a label and a description.
Question adds instructions, SaveTo keeps the category's label in a run
variable or a contact attribute, and MinConfidence (low, medium or
high) sends weaker answers down the none path.
*/
type ClassifySettings struct {
	Question      string `json:"question,omitempty"`
	SaveTo        string `json:"save_to,omitempty"`
	Source        string `json:"source,omitempty"`
	MinConfidence string `json:"min_confidence,omitempty"`
}

/*
WaitUntil names a moment to wait for: the schedule's next opening, or
the next time of day.
*/
type WaitUntil struct {
	Kind       string `json:"kind"`
	ScheduleID string `json:"schedule_id,omitempty"`
	Time       string `json:"time,omitempty"`
}

/*
WaitSettings pauses the run For a duration or Until a moment. Count
business counts only open hours of the schedule. InterruptOn lists what
cuts the wait short: customer_message (default) and teammate_reply.
*/
type WaitSettings struct {
	For         string     `json:"for,omitempty"`
	Until       *WaitUntil `json:"until,omitempty"`
	Count       string     `json:"count,omitempty"`
	ScheduleID  string     `json:"schedule_id,omitempty"`
	InterruptOn []string   `json:"interrupt_on,omitempty"`
}

/*
RunWorkflowSettings calls a reusable workflow and waits for it.
*/
type RunWorkflowSettings struct {
	WorkflowID string `json:"workflow_id"`
}

/*
AssignSettings assigns the conversation: to a member, to a team, or to
a team's member by round robin or by the lightest load. KeepIfAssigned
(default true) leaves a conversation a member already holds alone.
*/
type AssignSettings struct {
	Mode           string `json:"mode"`
	MemberID       string `json:"member_id,omitempty"`
	TeamID         string `json:"team_id,omitempty"`
	OnlyAvailable  *bool  `json:"only_available,omitempty"`
	Fallback       string `json:"fallback,omitempty"`
	KeepIfAssigned *bool  `json:"keep_if_assigned,omitempty"`
}
