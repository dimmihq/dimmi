/*
|--------------------------------------------------------------------------
| Purge What Retention Lets Go
|--------------------------------------------------------------------------
|
| Once a day, every workspace has its trash emptied, its spam deleted and
| the source of its received email removed, each after the days the
| workspace chose. A conversation purged this way is gone for good: its
| messages, their files, and the mail it received and sent. The source
| of an email goes while the message it became stays; mail that never
| became a message leaves no record at all. A period changed in the last
| day waits for the next round, and the retention guard is asked about
| every conversation first; a held one stays where it is.
|
*/

package retention

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
purgeBatch is how many conversations or emails one transaction removes.
*/
const purgeBatch = 100

/*
Disks opens the disks files are kept on.
*/
type Disks interface {
	/*
		Disk returns the disk with the name.
	*/
	Disk(name string) (filesystem.Filesystem, error)
}

/*
Purged counts what a round removed.
*/
type Purged struct {
	Trashed   int64
	Spam      int64
	RawEmails int64
}

/*
PurgeExpired removes what the workspaces' trash, spam and raw email
periods let go.
*/
type PurgeExpired struct {
	DB        *database.DB
	Clock     clock.Clock
	Storage   Disks
	Retention contracts.RetentionGuard
}

/*
storedFile is a file on a disk.
*/
type storedFile struct {
	disk string
	path string
}

/*
Handle runs the round over every live workspace and returns what it
removed.
*/
func (a *PurgeExpired) Handle(ctx context.Context) (Purged, error) {
	var total Purged

	workspaces, err := models.New(a.DB).ListWorkspaces(ctx)
	if err != nil {
		return total, fmt.Errorf("list workspaces: %w", err)
	}

	for _, w := range workspaces {
		got, err := a.Workspace(ctx, w)
		total.Trashed += got.Trashed
		total.Spam += got.Spam
		total.RawEmails += got.RawEmails

		if err != nil {
			return total, err
		}
	}

	return total, nil
}

/*
Workspace runs the round for one workspace.
*/
func (a *PurgeExpired) Workspace(ctx context.Context, w models.Workspace) (Purged, error) {
	var out Purged

	r, now := w.Retention(), a.Clock.Now()
	if r.ChangedAt != nil && now.Sub(*r.ChangedAt) < settled {
		return out, nil
	}

	ctx = tenancy.With(ctx, w.ID.String())

	var err error

	out.Trashed, err = a.conversations(ctx, w.ID, func(q *models.Queries, held []ids.UUID) ([]ids.UUID, error) {
		return q.ListTrashedConversationsBefore(ctx, models.ListTrashedConversationsBeforeParams{
			WorkspaceID: w.ID, Cutoff: now.Add(-time.Duration(r.TrashDays) * day), Skipped: held, RowLimit: purgeBatch,
		})
	})
	if err != nil {
		return out, fmt.Errorf("empty trash: %w", err)
	}

	out.Spam, err = a.conversations(ctx, w.ID, func(q *models.Queries, held []ids.UUID) ([]ids.UUID, error) {
		return q.ListSpamConversationsBefore(ctx, models.ListSpamConversationsBeforeParams{
			WorkspaceID: w.ID, Cutoff: now.Add(-time.Duration(r.SpamDays) * day), Skipped: held, RowLimit: purgeBatch,
		})
	})
	if err != nil {
		return out, fmt.Errorf("purge spam: %w", err)
	}

	out.RawEmails, err = a.rawEmails(ctx, w.ID, now.Add(-time.Duration(r.RawEmailDays)*day))
	if err != nil {
		return out, fmt.Errorf("purge raw email: %w", err)
	}

	return out, nil
}

/*
conversations deletes the conversations list returns, a batch at a time,
until none is left that the guard lets go, and returns how many went.
*/
func (a *PurgeExpired) conversations(
	ctx context.Context, workspaceID ids.UUID, list func(*models.Queries, []ids.UUID) ([]ids.UUID, error),
) (int64, error) {
	held := []ids.UUID{}

	var purged int64

	for {
		var found []ids.UUID

		err := a.DB.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
			var err error

			found, err = list(models.New(tx), held)

			return err
		})
		if err != nil {
			return purged, fmt.Errorf("list conversations: %w", err)
		}

		allowed, err := a.allowed(ctx, workspaceID, found, &held)
		if err != nil {
			return purged, err
		}

		count, err := a.destroy(ctx, workspaceID, allowed)
		purged += count

		if err != nil || len(found) < purgeBatch {
			return purged, err
		}
	}
}

/*
allowed returns the conversations the guard lets go and adds the rest to
held.
*/
func (a *PurgeExpired) allowed(
	ctx context.Context, workspaceID ids.UUID, found []ids.UUID, held *[]ids.UUID,
) ([]ids.UUID, error) {
	out := make([]ids.UUID, 0, len(found))

	for _, id := range found {
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

/*
destroy deletes the conversations' files, then their rows and the mail
they received and sent. Files go first: a run that stops between the
two finds the rows again and a file gone already is no error.
*/
func (a *PurgeExpired) destroy(ctx context.Context, workspaceID ids.UUID, conversations []ids.UUID) (int64, error) {
	if len(conversations) == 0 {
		return 0, nil
	}

	var (
		files []storedFile
		raws  []RawEmail
	)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		files, raws, err = conversationFiles(ctx, models.New(tx), workspaceID, conversations)

		return err
	})
	if err != nil {
		return 0, err
	}

	if err := a.deleteFiles(ctx, files); err != nil {
		return 0, err
	}

	if err := DeleteRawEmails(ctx, a.DB, a.Storage, raws); err != nil {
		return 0, err
	}

	return a.deleteRows(ctx, workspaceID, conversations)
}

/*
deleteRows deletes the conversations' rows and the records of the mail
they received and sent, and returns how many conversations went.
*/
func (a *PurgeExpired) deleteRows(ctx context.Context, workspaceID ids.UUID, conversations []ids.UUID) (int64, error) {
	var purged int64

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.DeleteConversationInboundEmails(ctx, models.DeleteConversationInboundEmailsParams{
			WorkspaceID: workspaceID, Ids: conversations,
		}); err != nil {
			return fmt.Errorf("delete received email: %w", err)
		}

		if _, err := q.DeleteConversationOutboundEmails(ctx, models.DeleteConversationOutboundEmailsParams{
			WorkspaceID: workspaceID, Ids: conversations,
		}); err != nil {
			return fmt.Errorf("delete sent email: %w", err)
		}

		var err error

		purged, err = q.PurgeConversations(ctx, models.PurgeConversationsParams{
			WorkspaceID: workspaceID, Ids: conversations,
		})
		if err != nil {
			return fmt.Errorf("delete conversations: %w", err)
		}

		return nil
	})

	return purged, err
}

/*
conversationFiles returns the attachments and the stored emails of the
conversations.
*/
func conversationFiles(
	ctx context.Context, q *models.Queries, workspaceID ids.UUID, conversations []ids.UUID,
) ([]storedFile, []RawEmail, error) {
	attachments, err := q.ListConversationFiles(ctx, models.ListConversationFilesParams{
		WorkspaceID: workspaceID, Ids: conversations,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list attachments: %w", err)
	}

	stored, err := q.ListConversationRawEmails(ctx, models.ListConversationRawEmailsParams{
		WorkspaceID: workspaceID, Ids: conversations,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list raw email: %w", err)
	}

	files := make([]storedFile, 0, len(attachments))
	for _, file := range attachments {
		files = append(files, storedFile{disk: file.Disk, path: file.Path})
	}

	raws := make([]RawEmail, 0, len(stored))
	for _, raw := range stored {
		raws = append(raws, RawEmail{ID: raw.ID, Disk: raw.RawDisk, Path: *raw.RawPath})
	}

	return files, raws, nil
}

/*
rawEmails removes the source of mail read before the cutoff, a batch at
a time, and returns how many emails it handled. The record of mail that
became a message stays without its source; the record of mail that did
not is deleted with it.
*/
func (a *PurgeExpired) rawEmails(ctx context.Context, workspaceID ids.UUID, cutoff time.Time) (int64, error) {
	var purged int64

	for {
		var rows []models.ListExpiredRawEmailsRow

		err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error

			rows, err = models.New(tx).ListExpiredRawEmails(ctx, models.ListExpiredRawEmailsParams{
				WorkspaceID: workspaceID, Cutoff: cutoff, RowLimit: purgeBatch,
			})

			return err
		})
		if err != nil {
			return purged, fmt.Errorf("list raw email: %w", err)
		}

		if err := a.forget(ctx, workspaceID, rows); err != nil {
			return purged, err
		}

		purged += int64(len(rows))

		if len(rows) < purgeBatch {
			return purged, nil
		}
	}
}

/*
forget deletes the emails' stored source and clears or deletes their
records.
*/
func (a *PurgeExpired) forget(ctx context.Context, workspaceID ids.UUID, rows []models.ListExpiredRawEmailsRow) error {
	if len(rows) == 0 {
		return nil
	}

	all := make([]ids.UUID, 0, len(rows))
	raws := make([]RawEmail, 0, len(rows))

	for _, row := range rows {
		all = append(all, row.ID)

		if row.RawPath != nil {
			raws = append(raws, RawEmail{ID: row.ID, Disk: row.RawDisk, Path: *row.RawPath})
		}
	}

	if err := DeleteRawEmails(ctx, a.DB, a.Storage, raws); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if _, err := q.DeleteInboundEmails(ctx, models.DeleteInboundEmailsParams{
			WorkspaceID: workspaceID, Ids: all,
		}); err != nil {
			return fmt.Errorf("delete email records: %w", err)
		}

		if _, err := q.ClearRawEmails(ctx, models.ClearRawEmailsParams{
			Now: a.Clock.Now(), WorkspaceID: workspaceID, Ids: all,
		}); err != nil {
			return fmt.Errorf("clear raw email: %w", err)
		}

		return nil
	})
}

/*
deleteFiles removes stored files, files gone already included.
*/
func (a *PurgeExpired) deleteFiles(ctx context.Context, files []storedFile) error {
	for _, file := range files {
		disk, err := a.Storage.Disk(file.disk)
		if err != nil {
			return fmt.Errorf("open disk %s: %w", file.disk, err)
		}

		if err := disk.Delete(ctx, file.path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
			return fmt.Errorf("delete file: %w", err)
		}
	}

	return nil
}
