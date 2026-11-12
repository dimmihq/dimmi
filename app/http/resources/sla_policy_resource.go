/*
|--------------------------------------------------------------------------
| SLA Policy Resource
|--------------------------------------------------------------------------
|
| A service level policy: its targets in seconds, null when unset, the
| schedule whose open hours they count in, the warning ahead of each
| target, when its clocks pause, what a breach changes, and the ticket
| types and priorities it applies to new tickets of.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SLAPolicy is one service level policy.
*/
type SLAPolicy struct {
	Object                 string     `json:"object"`
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	ScheduleID             *string    `json:"schedule_id"`
	FirstReplySeconds      *int32     `json:"first_reply_seconds"`
	NextReplySeconds       *int32     `json:"next_reply_seconds"`
	FollowUpSeconds        *int32     `json:"follow_up_seconds"`
	ResolutionSeconds      *int32     `json:"resolution_seconds"`
	WarnBeforeSeconds      int32      `json:"warn_before_seconds"`
	PauseOnWaitingCustomer bool       `json:"pause_on_waiting_customer"`
	PauseOnWaitingTeam     bool       `json:"pause_on_waiting_team"`
	BreachPriority         *string    `json:"breach_priority"`
	BreachTeamID           *string    `json:"breach_team_id"`
	TicketTypeIDs          []string   `json:"ticket_type_ids"`
	Priorities             []string   `json:"priorities"`
	Position               int32      `json:"position"`
	ArchivedAt             *time.Time `json:"archived_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

/*
NewSLAPolicies shapes policies.
*/
func NewSLAPolicies(list []models.SlaPolicy) []SLAPolicy {
	out := make([]SLAPolicy, len(list))
	for i, p := range list {
		out[i] = NewSLAPolicy(p)
	}

	return out
}

/*
NewSLAPolicy shapes one policy.
*/
func NewSLAPolicy(p models.SlaPolicy) SLAPolicy {
	priorities := p.Priorities
	if priorities == nil {
		priorities = []string{}
	}

	return SLAPolicy{
		Object: "sla_policy", ID: models.PublicID(models.PrefixSLAPolicy, p.ID), Name: p.Name,
		Description: p.Description, ScheduleID: optionalID(models.PrefixSchedule, p.ScheduleID),
		FirstReplySeconds: p.FirstReplySeconds, NextReplySeconds: p.NextReplySeconds,
		FollowUpSeconds: p.FollowUpSeconds, ResolutionSeconds: p.ResolutionSeconds,
		WarnBeforeSeconds: p.WarnBeforeSeconds, PauseOnWaitingCustomer: p.PauseOnWaitingCustomer,
		PauseOnWaitingTeam: p.PauseOnWaitingTeam, BreachPriority: p.BreachPriority,
		BreachTeamID:  optionalID(models.PrefixTeam, p.BreachTeamID),
		TicketTypeIDs: publicIDs(models.PrefixTicketType, p.TicketTypeIds), Priorities: priorities,
		Position: p.Position, ArchivedAt: p.ArchivedAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
