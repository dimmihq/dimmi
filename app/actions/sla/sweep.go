/*
|--------------------------------------------------------------------------
| Warning And Breaching
|--------------------------------------------------------------------------
|
| Every minute a sweep finds, across workspaces, the running clocks due
| to warn and the ones past due. Each is handled in its workspace's own
| transaction under a lock, so a clock warns once and breaches once. A
| ticket's clock announces ticket.sla_warning or ticket.sla_breached; a
| clock on a conversation, a plain one or a customer ticket's, starts
| the workflows of the SLA triggers. A breach raises the priority and
| moves the work to the policy's breach team when the policy says so.
|
*/

package sla

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
sweepBatch is how many clocks one sweep handles.
*/
const sweepBatch = 500

/*
Sweep warns about and breaches the clocks that are due.
*/
func (s *SLA) Sweep(ctx context.Context) error {
	var due []models.SLAClocksDueRow

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		due, err = models.New(tx).SLAClocksDue(ctx, models.SLAClocksDueParams{
			DueAt: s.Clock.Now(), RowLimit: sweepBatch,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list due sla clocks: %w", err)
	}

	for _, d := range due {
		if err := s.alert(ctx, d.WorkspaceID, d.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
alert warns about or breaches one clock, if it still runs and is due.
*/
func (s *SLA) alert(ctx context.Context, ws, id ids.UUID) error {
	ctx = tenancy.With(ctx, ws.String())

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		clock, err := q.LockSLAClock(ctx, models.LockSLAClockParams{WorkspaceID: ws, ID: id})
		if err != nil || clock.State != StateRunning {
			return nil //nolint:nilerr // a clock gone or stopped has nothing to say
		}

		now := s.Clock.Now()
		breached := !clock.DueAt.After(now)
		switch {
		case breached:
			clock.State, clock.BreachedAt = StateBreached, &now
		case clock.WarnedAt == nil && clock.WarnAt != nil && !clock.WarnAt.After(now):
			clock.WarnedAt = &now
		default:
			return nil
		}

		a := &alarm{s: s, q: q, tx: tx, clock: clock, breached: breached}
		if err := (clocks{q: q, now: now}).save(ctx, clock); err != nil {
			return err
		}

		return a.raise(ctx)
	})
}

/*
alarm is one clock warning or breaching, in its transaction.
*/
type alarm struct {
	s        *SLA
	q        *models.Queries
	tx       *database.Tx
	clock    models.SlaClock
	breached bool
	policy   models.SlaPolicy
	writer   *conversations.Writer
}

/*
raise announces the alarm and, on a breach, escalates the work.
*/
func (a *alarm) raise(ctx context.Context) error {
	p, err := findPolicy(ctx, a.q, a.clock.WorkspaceID, a.clock.PolicyID)
	if err != nil {
		return err
	}

	a.policy = p
	a.writer = &conversations.Writer{
		Q: a.q, Tx: a.tx, Events: a.s.Events, Now: a.s.Clock.Now(),
		Actor: conversations.Actor{Type: string(enums.AuthorTypeSystem), Name: "SLA: " + p.Name},
	}

	if a.clock.TicketID != nil {
		return a.onTicket(ctx, *a.clock.TicketID)
	}

	conv, err := a.q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: a.clock.WorkspaceID, ID: *a.clock.ConversationID,
	})
	if err != nil {
		return nil //nolint:nilerr // a conversation gone needs no alarm
	}

	if err := a.announceConversation(ctx, conv.ID); err != nil {
		return err
	}

	return a.escalateConversation(ctx, conv)
}

/*
higher reports whether the priority is above the current one, in the
order of the levels.
*/
func higher(levels []string, to, current string) bool {
	return slices.Index(levels, to) > slices.Index(levels, current)
}
