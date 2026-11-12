/*
|--------------------------------------------------------------------------
| SLA Policy Input
|--------------------------------------------------------------------------
|
| A policy's body becomes the action's input: public ids decoded, an
| empty schedule or breach team clearing it, and a breach priority of
| none clearing the escalation. An id that names nothing is refused on
| its field.
|
*/

package v1

import (
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/sla"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
input turns a policy body into the action's input.
*/
func (c *SLAController) input(b requests.SLAPolicyBody) (sla.Input, error) {
	in := sla.Input{
		Name: b.Name, Description: b.Description, FirstReply: b.FirstReplySeconds, NextReply: b.NextReplySeconds,
		FollowUp: b.FollowUpSeconds, Resolution: b.ResolutionSeconds, WarnBefore: b.WarnBeforeSeconds,
		PauseOnWaitingCustomer: b.PauseOnWaitingCustomer, PauseOnWaitingTeam: b.PauseOnWaitingTeam,
		BreachPriority: b.BreachPriority, Priorities: b.Priorities,
	}

	if b.BreachPriority != nil && *b.BreachPriority == "none" {
		in.BreachPriority = new(string)
	}

	var err error
	if in.ScheduleID, in.ClearSchedule, err = c.optional(models.PrefixSchedule, b.ScheduleID,
		sla.ErrScheduleNotFound); err != nil {
		return in, err
	}

	if in.BreachTeamID, in.ClearBreachTeam, err = c.optional(models.PrefixTeam, b.BreachTeamID,
		sla.ErrTeamNotFound); err != nil {
		return in, err
	}

	if b.TicketTypeIDs != nil {
		in.TicketTypeIDs = make([]ids.UUID, 0, len(b.TicketTypeIDs))
		for _, public := range b.TicketTypeIDs {
			id, err := c.IDs.Decode(string(models.PrefixTicketType), public)
			if err != nil {
				return in, sla.ErrTicketTypeNotFound
			}

			in.TicketTypeIDs = append(in.TicketTypeIDs, id)
		}
	}

	return in, nil
}

/*
optional decodes an optional id: nil when left out, cleared when empty.
*/
func (c *SLAController) optional(prefix models.Prefix, public *string, invalid error) (*ids.UUID, bool, error) {
	if public == nil {
		return nil, false, nil
	}

	if *public == "" {
		return nil, true, nil
	}

	id, err := c.IDs.Decode(string(prefix), *public)
	if err != nil {
		return nil, false, invalid
	}

	return &id, false, nil
}
