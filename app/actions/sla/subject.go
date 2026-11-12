/*
|--------------------------------------------------------------------------
| What A Clock Runs On
|--------------------------------------------------------------------------
|
| Clocks run on a ticket or on a plain conversation. A customer ticket
| with a policy owns the clocks of its conversation's replies; a
| conversation without one follows the policy a workflow applied to it.
| A subject carries its policy and the hours it counts in, read once.
|
*/

package sla

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
subject is a ticket or a plain conversation with the policy it follows.
For a ticket, conv is its conversation, nil for a ticket without one.
*/
type subject struct {
	ws     ids.UUID
	ticket *models.Ticket
	conv   *models.Conversation
	policy models.SlaPolicy
	hours  *officehours.Schedule
}

/*
keys returns the ticket or conversation id clocks are stored under.
*/
func (s *subject) keys() (ticket, conversation *ids.UUID) {
	if s.ticket != nil {
		return &s.ticket.ID, nil
	}

	return nil, &s.conv.ID
}

/*
waiting reports whether the policy pauses the subject's clocks now:
the ticket waits on the customer, or on the team when the policy says
so; a plain conversation is snoozed.
*/
func (s *subject) waiting() bool {
	p := s.policy
	if s.ticket != nil {
		return s.ticket.StateGroup == tickets.GroupWaitingCustomer && p.PauseOnWaitingCustomer ||
			s.ticket.StateGroup == tickets.GroupWaitingTeam && p.PauseOnWaitingTeam
	}

	return s.conv.Status == string(enums.ConversationStatusSnoozed) && p.PauseOnWaitingCustomer
}

/*
add returns the moment the duration ends from the start, counting only
the policy's open hours when it has a schedule.
*/
func (s *subject) add(from time.Time, d time.Duration) time.Time {
	if s.hours == nil {
		return from.Add(d)
	}

	due, err := s.hours.AddBusiness(from, d)
	if err != nil {
		return from.Add(d)
	}

	return due
}

/*
between returns the counted time from one moment to another.
*/
func (s *subject) between(from, to time.Time) time.Duration {
	if !to.After(from) {
		return 0
	}

	if s.hours == nil {
		return to.Sub(from)
	}

	return s.hours.BusinessBetween(from, to)
}

/*
ticketSubject returns the ticket's subject, nil when it follows no
live policy.
*/
func ticketSubject(ctx context.Context, q *models.Queries, t models.Ticket) (*subject, error) {
	if t.SlaPolicyID == nil || t.DeletedAt != nil {
		return nil, nil
	}

	s := &subject{ws: t.WorkspaceID, ticket: &t}
	if t.ConversationID != nil {
		c, err := q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: t.WorkspaceID, ID: *t.ConversationID,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("find ticket conversation: %w", err)
		}

		if err == nil {
			s.conv = &c
		}
	}

	return s.withPolicy(ctx, q, *t.SlaPolicyID)
}

/*
conversationSubject returns the subject a conversation's messages
count for: its customer ticket when that follows a policy, or else the
conversation with the policy a workflow applied; nil for neither.
*/
func conversationSubject(ctx context.Context, q *models.Queries, ws, conv ids.UUID) (*subject, error) {
	t, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{WorkspaceID: ws, ConversationID: &conv})
	if err == nil && t.SlaPolicyID != nil && t.DeletedAt == nil {
		return ticketSubject(ctx, q, t)
	}

	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("find conversation ticket: %w", err)
	}

	policy, err := q.ConversationSLAPolicyID(ctx, models.ConversationSLAPolicyIDParams{
		WorkspaceID: ws, ConversationID: conv,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find conversation sla policy: %w", err)
	}

	c, err := q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: conv})
	if err != nil {
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	return (&subject{ws: ws, conv: &c}).withPolicy(ctx, q, policy)
}

/*
withPolicy loads the subject's policy and its hours; an archived or
missing policy leaves no subject.
*/
func (s *subject) withPolicy(ctx context.Context, q *models.Queries, id ids.UUID) (*subject, error) {
	p, err := findPolicy(ctx, q, s.ws, id)
	if errors.Is(err, ErrPolicyNotFound) || err == nil && p.ArchivedAt != nil {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	s.policy = p
	if p.ScheduleID == nil {
		return s, nil
	}

	row, err := q.FindSchedule(ctx, models.FindScheduleParams{WorkspaceID: s.ws, ID: *p.ScheduleID})
	if err != nil {
		return s, nil //nolint:nilerr // a schedule gone counts calendar time
	}

	if sched, err := schedules.Read(row); err == nil {
		s.hours = sched
	}

	return s, nil
}

/*
seconds returns the policy's time for the target, nil when unset.
*/
func (s *subject) seconds(target string) *int32 {
	p := s.policy

	return map[string]*int32{
		TargetFirstReply: p.FirstReplySeconds, TargetNextReply: p.NextReplySeconds,
		TargetFollowUp: p.FollowUpSeconds, TargetResolution: p.ResolutionSeconds,
	}[target]
}
