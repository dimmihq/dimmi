/*
|--------------------------------------------------------------------------
| Checking Policies
|--------------------------------------------------------------------------
|
| A policy's input lands on the stored one, then the whole is checked:
| its name, at least one target in range, the warning, the breach
| priority, and that every schedule, team and ticket type it names
| belongs to the workspace.
|
*/

package sla

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of a policy's times, in seconds.
*/
const (
	minTarget  = 60
	maxTarget  = 365 * 24 * 3600
	maxWarning = 7 * 24 * 3600
)

/*
withInput lands the input on a policy.
*/
func withInput(p models.SlaPolicy, in Input) models.SlaPolicy {
	if in.Name != nil {
		p.Name = strings.TrimSpace(*in.Name)
	}

	if in.Description != nil {
		p.Description = strings.TrimSpace(*in.Description)
	}

	p.ScheduleID = pick(p.ScheduleID, in.ScheduleID, in.ClearSchedule)
	p.BreachTeamID = pick(p.BreachTeamID, in.BreachTeamID, in.ClearBreachTeam)
	p.FirstReplySeconds = target(p.FirstReplySeconds, in.FirstReply)
	p.NextReplySeconds = target(p.NextReplySeconds, in.NextReply)
	p.FollowUpSeconds = target(p.FollowUpSeconds, in.FollowUp)
	p.ResolutionSeconds = target(p.ResolutionSeconds, in.Resolution)

	if in.WarnBefore != nil {
		p.WarnBeforeSeconds = *in.WarnBefore
	}

	if in.PauseOnWaitingCustomer != nil {
		p.PauseOnWaitingCustomer = *in.PauseOnWaitingCustomer
	}

	if in.PauseOnWaitingTeam != nil {
		p.PauseOnWaitingTeam = *in.PauseOnWaitingTeam
	}

	if in.BreachPriority != nil {
		p.BreachPriority = in.BreachPriority
		if *in.BreachPriority == "" {
			p.BreachPriority = nil
		}
	}

	return withLists(p, in)
}

/*
withLists lands the ticket types and priorities the policy matches.
*/
func withLists(p models.SlaPolicy, in Input) models.SlaPolicy {
	if in.TicketTypeIDs != nil {
		p.TicketTypeIds = in.TicketTypeIDs
	}

	if in.Priorities != nil {
		p.Priorities = in.Priorities
	}

	if p.TicketTypeIds == nil {
		p.TicketTypeIds = []ids.UUID{}
	}

	if p.Priorities == nil {
		p.Priorities = []string{}
	}

	return p
}

/*
pick returns the new id, nil when cleared, or the stored one.
*/
func pick(stored, asked *ids.UUID, cleared bool) *ids.UUID {
	switch {
	case cleared:
		return nil
	case asked != nil:
		return asked
	}

	return stored
}

/*
target returns the new target, nil when zero, or the stored one.
*/
func target(stored, asked *int32) *int32 {
	switch {
	case asked == nil:
		return stored
	case *asked == 0:
		return nil
	}

	return asked
}

/*
check refuses a policy that cannot work.
*/
func check(ctx context.Context, q *models.Queries, ws ids.UUID, p models.SlaPolicy) error {
	if n := utf8.RuneCountInString(p.Name); n < 1 || n > 80 {
		return ErrNameInvalid
	}

	if utf8.RuneCountInString(p.Description) > 500 {
		return ErrDescriptionInvalid
	}

	if err := checkTargets(p); err != nil {
		return err
	}

	if p.WarnBeforeSeconds < 0 || p.WarnBeforeSeconds > maxWarning {
		return ErrWarnInvalid
	}

	if p.BreachPriority != nil && !slices.Contains([]string{"low", "medium", "high", "urgent"}, *p.BreachPriority) {
		return ErrPriorityInvalid
	}

	for _, pr := range p.Priorities {
		if !slices.Contains(tickets.Priorities(), pr) {
			return ErrPriorityInvalid.OnField("priorities")
		}
	}

	return checkRefs(ctx, q, ws, p)
}

/*
checkTargets refuses a policy without a target or with one out of range.
*/
func checkTargets(p models.SlaPolicy) error {
	set := 0
	for field, v := range map[string]*int32{
		"first_reply_seconds": p.FirstReplySeconds, "next_reply_seconds": p.NextReplySeconds,
		"follow_up_seconds": p.FollowUpSeconds, "resolution_seconds": p.ResolutionSeconds,
	} {
		if v == nil {
			continue
		}

		set++
		if *v < minTarget || *v > maxTarget {
			return ErrTargetInvalid.OnField(field)
		}
	}

	if set == 0 {
		return ErrNoTarget
	}

	return nil
}

/*
checkRefs refuses schedules, teams and ticket types of no workspace's
own.
*/
func checkRefs(ctx context.Context, q *models.Queries, ws ids.UUID, p models.SlaPolicy) error {
	if p.ScheduleID != nil {
		_, err := q.FindSchedule(ctx, models.FindScheduleParams{WorkspaceID: ws, ID: *p.ScheduleID})
		if err := missing(err, ErrScheduleNotFound); err != nil {
			return err
		}
	}

	if p.BreachTeamID != nil {
		_, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: ws, ID: *p.BreachTeamID})
		if err := missing(err, ErrTeamNotFound); err != nil {
			return err
		}
	}

	for _, id := range p.TicketTypeIds {
		_, err := q.FindTicketType(ctx, models.FindTicketTypeParams{WorkspaceID: ws, ID: id})
		if err := missing(err, ErrTicketTypeNotFound); err != nil {
			return err
		}
	}

	return nil
}

/*
missing turns a row not found into the failure, and wraps any other.
*/
func missing(err, notFound error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, database.ErrNotFound):
		return notFound
	}

	return fmt.Errorf("check sla policy reference: %w", err)
}
