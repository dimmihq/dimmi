/*
|--------------------------------------------------------------------------
| SLA Requests
|--------------------------------------------------------------------------
|
| The inputs of service level policies and the clocks they run. Targets
| are seconds; on a change, a target of 0 removes it, an empty schedule
| or breach team removes that, and a breach priority of none removes
| the escalation. Ids are the public ids the API answers with.
|
*/

package requests

/*
SLAPolicyRequest names a policy in the path.
*/
type SLAPolicyRequest struct {
	Policy string `path:"policy"`
}

/*
SLAPolicyBody is what a policy holds; fields left out keep what is
stored.
*/
type SLAPolicyBody struct {
	Name                   *string  `json:"name,omitempty"                      maxLength:"80"`
	Description            *string  `json:"description,omitempty"               maxLength:"500"`
	ScheduleID             *string  `json:"schedule_id,omitempty"               maxLength:"64"`
	FirstReplySeconds      *int32   `json:"first_reply_seconds,omitempty"       minimum:"0" maximum:"31536000"`
	NextReplySeconds       *int32   `json:"next_reply_seconds,omitempty"        minimum:"0" maximum:"31536000"`
	FollowUpSeconds        *int32   `json:"follow_up_seconds,omitempty"         minimum:"0" maximum:"31536000"`
	ResolutionSeconds      *int32   `json:"resolution_seconds,omitempty"        minimum:"0" maximum:"31536000"`
	WarnBeforeSeconds      *int32   `json:"warn_before_seconds,omitempty"       minimum:"0" maximum:"604800"`
	PauseOnWaitingCustomer *bool    `json:"pause_on_waiting_customer,omitempty"`
	PauseOnWaitingTeam     *bool    `json:"pause_on_waiting_team,omitempty"`
	BreachPriority         *string  `json:"breach_priority,omitempty"           enum:"none,low,medium,high,urgent"`
	BreachTeamID           *string  `json:"breach_team_id,omitempty"            maxLength:"64"`
	TicketTypeIDs          []string `json:"ticket_type_ids,omitempty"           maxItems:"50"`
	Priorities             []string `json:"priorities,omitempty"                maxItems:"5"`
}

/*
StoreSLAPolicyRequest is the input of POST /sla-policies.
*/
type StoreSLAPolicyRequest struct {
	Body SLAPolicyBody
}

/*
UpdateSLAPolicyRequest is the input of PATCH /sla-policies/{policy}.
*/
type UpdateSLAPolicyRequest struct {
	Policy string `path:"policy"`
	Body   SLAPolicyBody
}

/*
ConversationSLAClocksRequest names a conversation whose clocks to list.
*/
type ConversationSLAClocksRequest struct {
	Conversation string `path:"conversation"`
}
