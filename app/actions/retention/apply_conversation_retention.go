/*
|--------------------------------------------------------------------------
| Apply Conversation Retention
|--------------------------------------------------------------------------
|
| Once a day, every workspace that set a period for closed conversations
| has those closed longer than that, with nothing new since, moved to the
| trash, where the trash period then takes over. A period set or changed
| in the last day waits for the next round, so shortening it never
| removes anything the day it is chosen. Each conversation is put to the
| retention guard first; a held one stays where it is.
|
*/

package retention

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of the round: the conversations one transaction moves, and how
long a changed period waits.
*/
const (
	batch   = 500
	settled = 24 * time.Hour
	day     = 24 * time.Hour
)

/*
ApplyConversationRetention moves stale closed conversations to the
trash.
*/
type ApplyConversationRetention struct {
	DB        *database.DB
	Clock     clock.Clock
	Retention contracts.RetentionGuard
}

/*
Handle runs the round over every workspace that set a period and
returns how many conversations it moved.
*/
func (a *ApplyConversationRetention) Handle(ctx context.Context) (int64, error) {
	workspaces, err := models.New(a.DB).ListRetainingWorkspaces(ctx)
	if err != nil {
		return 0, fmt.Errorf("list workspaces: %w", err)
	}

	var moved int64

	for _, w := range workspaces {
		count, err := a.workspace(ctx, w)
		if err != nil {
			return moved, err
		}

		moved += count
	}

	return moved, nil
}

/*
workspace moves the workspace's stale conversations, a batch per
transaction, until none is left that the guard lets go.
*/
func (a *ApplyConversationRetention) workspace(ctx context.Context, w models.Workspace) (int64, error) {
	r := w.Retention()
	now := a.Clock.Now()

	if r.ClosedConversationDays == nil || (r.ChangedAt != nil && now.Sub(*r.ChangedAt) < settled) {
		return 0, nil
	}

	ctx = tenancy.With(ctx, w.ID.String())
	cutoff := now.Add(-time.Duration(*r.ClosedConversationDays) * day)
	held := []ids.UUID{}

	var moved int64

	for {
		count, found, err := a.batch(ctx, w.ID, cutoff, &held)
		if err != nil {
			return moved, err
		}

		moved += count

		if found < batch {
			return moved, nil
		}
	}
}

/*
batch moves one batch and returns how many it moved and how many it
looked at, adding those the guard holds to held.
*/
func (a *ApplyConversationRetention) batch(
	ctx context.Context, workspaceID ids.UUID, cutoff time.Time, held *[]ids.UUID,
) (int64, int, error) {
	var (
		moved int64
		found int
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		stale, err := q.ListStaleClosedConversations(ctx, models.ListStaleClosedConversationsParams{
			WorkspaceID: workspaceID, Cutoff: cutoff, Skipped: *held, RowLimit: batch,
		})
		if err != nil {
			return fmt.Errorf("list stale conversations: %w", err)
		}

		found = len(stale)

		allowed, err := a.allowed(ctx, workspaceID, stale, held)
		if err != nil || len(allowed) == 0 {
			return err
		}

		moved, err = q.TrashStaleConversations(ctx, models.TrashStaleConversationsParams{
			Now: a.Clock.Now(), WorkspaceID: workspaceID, Ids: allowed,
		})
		if err != nil {
			return fmt.Errorf("trash conversations: %w", err)
		}

		return nil
	})

	return moved, found, err
}

/*
allowed returns the conversations the guard lets go and adds the rest to
held.
*/
func (a *ApplyConversationRetention) allowed(
	ctx context.Context, workspaceID ids.UUID, stale []ids.UUID, held *[]ids.UUID,
) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(stale))

	for _, id := range stale {
		ok, err := a.Retention.MayDestroy(ctx, workspaceID, contracts.RetentionSubject{
			Kind: contracts.RetentionConversation, ID: id,
		})
		if err != nil {
			return nil, fmt.Errorf("ask retention guard: %w", err)
		}

		if !ok {
			*held = append(*held, id)

			continue
		}

		out = append(out, id)
	}

	return out, nil
}
