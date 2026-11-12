/*
|--------------------------------------------------------------------------
| Service Levels
|--------------------------------------------------------------------------
|
| A policy promises targets: the first reply, every next reply, a
| follow-up when the team spoke last, and resolution. A ticket follows
| the policy set on it or the first policy matching its type and
| priority; a plain conversation follows the policy a workflow applied.
| Each target runs as a clock that counts only the policy's open hours,
| pauses while the work waits on the customer, and ends met, breached
| or canceled. A sweep warns before a clock is due and breaches it when
| it is, raising the priority and moving the work to a team when the
| policy says so.
|
*/

package sla

import (
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/tickets"
)

/*
The targets a clock runs for.
*/
const (
	TargetFirstReply = "first_reply"
	TargetNextReply  = "next_reply"
	TargetFollowUp   = "follow_up"
	TargetResolution = "resolution"
)

/*
The states of a clock.
*/
const (
	StateRunning  = "running"
	StatePaused   = "paused"
	StateMet      = "met"
	StateBreached = "breached"
	StateCanceled = "canceled"
)

/*
Targets returns every target in the order work meets them.
*/
func Targets() []string {
	return []string{TargetFirstReply, TargetNextReply, TargetFollowUp, TargetResolution}
}

/*
replyTargets are the targets a reply or a resolve ends.
*/
func replyTargets() []string {
	return []string{TargetFirstReply, TargetNextReply, TargetFollowUp}
}

/*
Failures of working service levels.
*/
var (
	ErrPolicyNotFound = fault.New(fault.NotFound, "sla_policy_not_found",
		"No service level policy of this workspace has this id.")
	ErrNameInvalid        = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 80 characters.").OnField("name")
	ErrDescriptionInvalid = fault.New(fault.Invalid, "description_invalid",
		"Descriptions are 500 characters at most.").OnField("description")
	ErrNoTarget = fault.New(fault.Invalid, "sla_target_missing",
		"Set at least one target: first reply, next reply, follow-up or resolution.").OnField("first_reply_seconds")
	ErrTargetInvalid = fault.New(fault.Invalid, "sla_target_invalid",
		"Targets are one minute to one year.")
	ErrWarnInvalid = fault.New(fault.Invalid, "warn_before_invalid",
		"Warn up to seven days before a target.").OnField("warn_before_seconds")
	ErrPriorityInvalid = fault.New(fault.Invalid, "priority_invalid",
		"Use low, medium, high or urgent.").OnField("breach_priority")
	ErrScheduleNotFound = fault.New(fault.Invalid, "schedule_not_found",
		"No schedule of this workspace has this id.").OnField("schedule_id")
	ErrTeamNotFound = fault.New(fault.Invalid, "team_not_found",
		"No team of this workspace has this id.").OnField("breach_team_id")
	ErrTicketTypeNotFound = fault.New(fault.Invalid, "ticket_type_not_found",
		"No ticket type of this workspace has this id.").OnField("ticket_type_ids")
	ErrPolicyArchived = fault.New(fault.Invalid, "sla_policy_archived",
		"An archived policy applies to nothing new.").OnField("sla_policy_id")
)

/*
SLA works service level policies and their clocks. Tickets is the
tickets action, whose writer escalates and applies policies to tickets.
*/
type SLA struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	Tickets *tickets.Tickets
}
