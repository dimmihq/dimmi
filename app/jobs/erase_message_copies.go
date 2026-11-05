/*
|--------------------------------------------------------------------------
| Erase Message Copies
|--------------------------------------------------------------------------
|
| Once a message is deleted, its attached files and every copy other
| parts of the product keep must go too, or "remove the card number"
| would leave it in the raw email. The job deletes the files, then runs
| each registered eraser, and is tried again until all of them finished.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
eraseAttempts is how often erasing is tried before it is given up.
*/
const eraseAttempts = 10

/*
MessageErasers are the erasers registered by the parts of the product
that keep copies of messages.
*/
type MessageErasers struct {
	hooks []contracts.MessageEraser
}

/*
Register adds an eraser.
*/
func (e *MessageErasers) Register(hook contracts.MessageEraser) {
	e.hooks = append(e.hooks, hook)
}

/*
EraseMessage runs every eraser and reports each that failed.
*/
func (e *MessageErasers) EraseMessage(ctx context.Context, workspaceID, messageID ids.UUID) error {
	var errs []error
	for _, hook := range e.hooks {
		errs = append(errs, hook.EraseMessage(ctx, workspaceID, messageID))
	}

	return errors.Join(errs...)
}

/*
EraseMessageCopiesArgs names the deleted message.
*/
type EraseMessageCopiesArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	MessageID   ids.UUID `json:"message_id"`
}

/*
Kind names the job.
*/
func (EraseMessageCopiesArgs) Kind() string {
	return "erase_message_copies"
}

/*
Options try the job until every copy is gone, once per message.
*/
func (EraseMessageCopiesArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: eraseAttempts, Unique: queue.UniqueOpts{ByArgs: true}}
}

/*
EraseMessageCopies deletes a deleted message's files and copies.
*/
type EraseMessageCopies struct {
	DB      *database.DB
	Storage *filesystem.Manager
	Erasers *MessageErasers
}

/*
Handle deletes the files, then runs the erasers.
*/
func (w *EraseMessageCopies) Handle(ctx context.Context, job *queue.Job[EraseMessageCopiesArgs]) error {
	args := job.Args
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	var files []models.Attachment

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		files, err = models.New(tx).ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
			WorkspaceID: args.WorkspaceID, MessageIds: []ids.UUID{args.MessageID},
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list attachments: %w", err)
	}

	for _, file := range files {
		if err := w.remove(ctx, file); err != nil {
			return err
		}
	}

	if err := w.Erasers.EraseMessage(ctx, args.WorkspaceID, args.MessageID); err != nil {
		return fmt.Errorf("erase copies: %w", err)
	}

	return nil
}

/*
remove deletes an attachment's file, already gone counting as done,
then its row.
*/
func (w *EraseMessageCopies) remove(ctx context.Context, file models.Attachment) error {
	disk, err := w.Storage.Disk(file.Disk)
	if err != nil {
		return fmt.Errorf("open disk %s: %w", file.Disk, err)
	}

	if err := disk.Delete(ctx, file.Path); err != nil && !errors.Is(err, filesystem.ErrNotFound) {
		return fmt.Errorf("delete attachment file: %w", err)
	}

	err = w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).DeleteAttachment(ctx, models.DeleteAttachmentParams{
			WorkspaceID: file.WorkspaceID, ID: file.ID,
		})
	})
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}

	return nil
}
