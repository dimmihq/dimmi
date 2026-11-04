/*
|--------------------------------------------------------------------------
| Erase a Contact
|--------------------------------------------------------------------------
|
| The wipe a request schedules. The retention guard is asked first: a
| held customer stays hidden and untouched, and the request waits as
| held until the job runs again. Otherwise every message of the
| customer's conversations loses its text and files, every registered
| eraser removes what its part kept, and the conversations are handed to
| the workspace's placeholder before the contact itself is deleted. Each
| step finds nothing to do when it already ran, so a failed attempt is
| simply tried again.
|
*/

package erasures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/contracts"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Outcome is where a run left a request: done, closed or gone already,
held by a retention hold, or waiting for its grace period to end.
*/
type Outcome int

/*
The outcomes of a run.
*/
const (
	Done Outcome = iota
	Held
	Waiting
)

/*
EraseContact carries out a due request.
*/
type EraseContact struct {
	DB        *database.DB
	Events    *events.Dispatcher
	Clock     clock.Clock
	Storage   *filesystem.Manager
	Retention contracts.RetentionGuard
	Erasers   *Erasers
}

/*
Handle erases the request's customer, or records the hold that keeps
it, and says where the request stands. A request closed or gone is
done; one whose grace period has not ended waits.
*/
func (a *EraseContact) Handle(ctx context.Context, args EraseArgs) (Outcome, error) {
	row, err := find(ctx, models.New(a.DB), args.WorkspaceID, args.ErasureID)
	if errors.Is(err, exceptions.ErrNotFound) || (err == nil && !open(row)) {
		return Done, nil
	}

	if err != nil {
		return Done, err
	}

	if row.ScheduledFor.After(a.Clock.Now()) {
		return Waiting, nil
	}

	may, err := a.Retention.MayDestroy(ctx, row.WorkspaceID, contracts.RetentionSubject{
		Kind: contracts.RetentionContact, ID: row.ContactID,
	})
	if err != nil {
		return Done, fmt.Errorf("ask retention guard: %w", err)
	}

	if !may {
		return Held, a.hold(ctx, row)
	}

	counts, err := a.wipe(ctx, row)
	if err != nil {
		return Done, err
	}

	if err := a.Erasers.Run(ctx, row.WorkspaceID, row.ContactID); err != nil {
		return Done, err
	}

	return Done, a.finish(ctx, row, counts)
}

/*
hold records that a retention hold keeps the customer, once.
*/
func (a *EraseContact) hold(ctx context.Context, row models.ContactErasure) error {
	if row.Status == "pending_hold" {
		return nil
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := a.Clock.Now()

		if _, err := models.New(tx).SetContactErasureStatus(ctx, models.SetContactErasureStatusParams{
			Status: "pending_hold", Now: now, WorkspaceID: row.WorkspaceID, ID: row.ID,
		}); err != nil {
			return fmt.Errorf("hold erasure: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactErasureHeld{
			Base: systemBase(row.WorkspaceID, now), ErasureID: row.ID, ContactID: row.ContactID,
		})
	})
}

/*
wipe deletes the files of the customer's conversations and empties
their messages, returning how many of each it removed.
*/
func (a *EraseContact) wipe(ctx context.Context, row models.ContactErasure) (map[string]int64, error) {
	q := models.New(a.DB)
	scope := models.ListErasedMessageFilesParams{WorkspaceID: row.WorkspaceID, ContactID: row.ContactID}

	files, err := q.ListErasedMessageFiles(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	for _, file := range files {
		if err := a.deleteFile(ctx, file.Disk, file.Path); err != nil {
			return nil, err
		}
	}

	counts := map[string]int64{}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		tq := models.New(tx)

		attachments, err := tq.DeleteErasedMessageFiles(ctx, models.DeleteErasedMessageFilesParams(scope))
		if err != nil {
			return fmt.Errorf("delete attachments: %w", err)
		}

		messages, err := tq.WipeErasedMessages(ctx, models.WipeErasedMessagesParams{
			Now: a.Clock.Now(), WorkspaceID: row.WorkspaceID, ContactID: row.ContactID,
		})
		if err != nil {
			return fmt.Errorf("wipe messages: %w", err)
		}

		counts["attachments"], counts["messages"] = attachments, messages

		return nil
	})

	return counts, err
}

/*
deleteFile removes a stored file, a file gone already included.
*/
func (a *EraseContact) deleteFile(ctx context.Context, disk, path string) error {
	fs, err := a.Storage.Disk(disk)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", disk, err)
	}

	if err := fs.Delete(ctx, path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

/*
finish hands the conversations to the placeholder, deletes the contact
and closes the request, all at once.
*/
func (a *EraseContact) finish(ctx context.Context, row models.ContactErasure, counts map[string]int64) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now := models.New(tx), a.Clock.Now()

		placeholder, err := q.CreatePlaceholderContact(ctx, models.CreatePlaceholderContactParams{
			WorkspaceID: row.WorkspaceID, Now: &now,
		})
		if err != nil {
			return fmt.Errorf("find placeholder: %w", err)
		}

		if counts["conversations"], err = q.HandOverErasedConversations(ctx, models.HandOverErasedConversationsParams{
			PlaceholderID: placeholder.ID, Now: now, WorkspaceID: row.WorkspaceID, ContactID: row.ContactID,
		}); err != nil {
			return fmt.Errorf("hand over conversations: %w", err)
		}

		if _, err := q.DeleteErasedContact(ctx, models.DeleteErasedContactParams{
			WorkspaceID: row.WorkspaceID, ID: row.ContactID,
		}); err != nil {
			return fmt.Errorf("delete contact: %w", err)
		}

		return a.complete(ctx, tx, row, counts)
	})
}

/*
complete closes the request with its counts and announces the erasure.
*/
func (a *EraseContact) complete(
	ctx context.Context, tx *database.Tx, row models.ContactErasure, counts map[string]int64,
) error {
	now := a.Clock.Now()

	encoded, err := countsJSON(counts)
	if err != nil {
		return err
	}

	if _, err := models.New(tx).CompleteContactErasure(ctx, models.CompleteContactErasureParams{
		Now: &now, Counts: encoded, WorkspaceID: row.WorkspaceID, ID: row.ID,
	}); err != nil {
		return fmt.Errorf("complete erasure: %w", err)
	}

	return a.Events.Dispatch(ctx, tx, appevents.ContactErased{
		Base: systemBase(row.WorkspaceID, now), ErasureID: row.ID, ContactID: row.ContactID, Counts: counts,
	})
}

/*
countsJSON encodes the counts for the request's record.
*/
func countsJSON(counts map[string]int64) (json.RawMessage, error) {
	encoded, err := json.Marshal(counts)
	if err != nil {
		return nil, fmt.Errorf("encode counts: %w", err)
	}

	return encoded, nil
}
