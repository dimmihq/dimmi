/*
|--------------------------------------------------------------------------
| Sending, Pausing And Stopping
|--------------------------------------------------------------------------
|
| Pressing send checks everything the broadcast needs, then either
| starts it or holds it for its time. A broadcast in each person's own
| time zone starts a day early, so the first zones get it on time. A
| pause keeps everyone still queued for later; a stop takes them off for
| good. A scheduled broadcast can go back to being a draft until it
| starts.
|
*/

package broadcasts

import (
	"context"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
localLead is how early a broadcast sent in each person's own zone
starts, so the earliest zones get it on time.
*/
const localLead = 26 * time.Hour

/*
ErrStatus refuses a change the broadcast's status does not allow.
*/
var ErrStatus = fault.New(fault.Conflict, "broadcast_status_conflict",
	"This broadcast can't do that in its current state.")

/*
Send starts the broadcast now, or holds it for its scheduled time.
*/
func (b *Broadcasts) Send(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	if !m.Can("broadcasts:send") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	var out models.Broadcast

	err := b.locked(ctx, m.Workspace.ID, id, func(ctx context.Context, tx *database.Tx, row models.Broadcast) error {
		if row.Status != StatusDraft {
			return ErrNotDraft
		}

		if err := b.ready(ctx, models.New(tx), row); err != nil {
			return err
		}

		status, at := StatusSending, b.Clock.Now()
		if row.ScheduleKind != "now" && row.ScheduledAt != nil {
			status, at = StatusScheduled, startAt(row, b.Clock.Now())
		}

		var err error

		out, err = b.move(ctx, tx, row, move{status: status, by: &m.Member.ID})
		if err != nil {
			return err
		}

		args := jobs.StartBroadcastArgs{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID}
		if status == StatusSending {
			return b.Queue.PushTx(ctx, tx, args)
		}

		return b.Queue.PushTx(ctx, tx, args, queue.At(at))
	})

	return out, err
}

/*
Pause holds a sending broadcast; everyone still queued waits.
*/
func (b *Broadcasts) Pause(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	return b.transition(ctx, m, id, step{from: []string{StatusSending}, to: StatusPaused, why: "paused_by_teammate"})
}

/*
Resume sends a paused broadcast on.
*/
func (b *Broadcasts) Resume(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	return b.transition(ctx, m, id, step{from: []string{StatusPaused}, to: StatusSending})
}

/*
Cancel turns a scheduled broadcast back into a draft.
*/
func (b *Broadcasts) Cancel(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	return b.transition(ctx, m, id, step{from: []string{StatusScheduled}, to: StatusDraft})
}

/*
Stop ends a broadcast for good; nobody still queued gets it.
*/
func (b *Broadcasts) Stop(ctx context.Context, m authz.Membership, id ids.UUID) (models.Broadcast, error) {
	return b.transition(ctx, m, id, step{
		from: []string{StatusScheduled, StatusSending, StatusPaused}, to: StatusStopped,
	})
}

/*
step is a status change a teammate asks for: the statuses it may start
from, the one it ends in and why.
*/
type step struct {
	from []string
	to   string
	why  string
}

/*
transition moves a broadcast between statuses on a teammate's say.
*/
func (b *Broadcasts) transition(
	ctx context.Context, m authz.Membership, id ids.UUID, st step,
) (models.Broadcast, error) {
	if !m.Can("broadcasts:send") {
		return models.Broadcast{}, exceptions.ErrForbidden
	}

	var out models.Broadcast

	err := b.locked(ctx, m.Workspace.ID, id, func(ctx context.Context, tx *database.Tx, row models.Broadcast) error {
		if !slices.Contains(st.from, row.Status) {
			return ErrStatus
		}

		var err error
		if out, err = b.move(ctx, tx, row, move{status: st.to, reason: st.why, actor: &m.Member.ID}); err != nil {
			return err
		}

		return b.afterMove(ctx, tx, out)
	})

	return out, err
}

/*
afterMove does what a new status needs: a resumed broadcast dispatches
again, a stopped one takes everyone queued off.
*/
func (b *Broadcasts) afterMove(ctx context.Context, tx *database.Tx, row models.Broadcast) error {
	switch row.Status {
	case StatusSending:
		return b.Queue.PushTx(ctx, tx, jobs.DispatchBroadcastArgs{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID})
	case StatusStopped:
		_, err := models.New(tx).SkipQueuedBroadcastRecipients(ctx, models.SkipQueuedBroadcastRecipientsParams{
			WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, Reason: new(ReasonBroadcastEnded), Now: b.Clock.Now(),
		})
		if err != nil {
			return wrap("skip queued recipients", err)
		}

		return b.Queue.PushTx(ctx, tx, jobs.TallyBroadcastArgs{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID})
	}

	return nil
}

/*
move is what changes with a status: the new status, who sent it, why
it paused and the teammate who moved it, nil for the system.
*/
type move struct {
	status string
	by     *ids.UUID
	reason string
	actor  *ids.UUID
}

/*
move stores a new status and tells open screens.
*/
func (b *Broadcasts) move(
	ctx context.Context, tx *database.Tx, row models.Broadcast, mv move,
) (models.Broadcast, error) {
	now := b.Clock.Now()
	status := mv.status
	params := models.SetBroadcastStatusParams{
		WorkspaceID: row.WorkspaceID, ID: row.ID, Status: status, SentByMemberID: mv.by, Now: now,
	}

	if mv.reason != "" {
		params.PauseReason = &mv.reason
	}

	if status == StatusSending {
		params.StartedAt = &now
	}

	if status == StatusSent || status == StatusStopped {
		params.FinishedAt = &now
	}

	out, err := models.New(tx).SetBroadcastStatus(ctx, params)
	if err != nil {
		return out, wrap("move broadcast", err)
	}

	if out.Status != row.Status {
		if err := b.milestone(ctx, tx, out, mv.actor); err != nil {
			return out, err
		}
	}

	return out, b.changed(ctx, tx, out)
}

/*
locked runs fn on the broadcast, locked, in a workspace transaction.
*/
func (b *Broadcasts) locked(ctx context.Context, ws, id ids.UUID,
	fn func(context.Context, *database.Tx, models.Broadcast) error,
) error {
	return b.DB.Transaction(withTenant(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).LockBroadcast(ctx, models.LockBroadcastParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return notFound(err, "lock broadcast")
		}

		return fn(ctx, tx, row)
	})
}

/*
startAt is when a scheduled broadcast starts: at its time, or a day
early for one sent in each person's own zone.
*/
func startAt(row models.Broadcast, now time.Time) time.Time {
	at := *row.ScheduledAt
	if row.ScheduleKind == "local" {
		at = at.Add(-localLead)
	}

	if at.Before(now) {
		return now
	}

	return at
}

/*
withTenant scopes the context to the workspace.
*/
func withTenant(ctx context.Context, ws ids.UUID) context.Context {
	return tenancy.With(ctx, ws.String())
}
