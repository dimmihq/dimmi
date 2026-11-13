/*
|--------------------------------------------------------------------------
| Scan Attachment
|--------------------------------------------------------------------------
|
| Every file that arrives, whether a teammate or a visitor uploaded it or
| an email carried it, is recorded as pending and handed to this job. The
| job reads the file from its disk and gives it to the malware scanner:
| a clean file is released to everyone, an infected one is blocked for
| good, and one the scanner cannot judge, such as a file over its size
| limit, is released as not scanned.
|
| A scanner that is down must not hide customers' files, so a scan that
| keeps failing is retried a few times with a growing delay and then
| gives way: the file is released as not scanned. Only a scanner that
| found a threat blocks a file. A file whose bytes are gone has nothing
| to scan and is settled the same way.
|
| A scan can also be lost with the job that carried it, so a sweep every
| few minutes queues the scans of files that have waited too long.
|
*/

package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/malware"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of scanning: how often a scan is tried, how long a file may wait
before the sweep queues its scan again, and how many files one sweep
queues.
*/
const (
	scanAttempts   = 5
	scanStaleAfter = 5 * time.Minute
	scanSweepBatch = 500
)

/*
The scan statuses a file ends in.
*/
const (
	scanClean    = "clean"
	scanInfected = "infected"
	scanSkipped  = "skipped"
)

/*
ScanAttachmentArgs names the file to scan.
*/
type ScanAttachmentArgs struct {
	WorkspaceID  ids.UUID `json:"workspace_id"`
	AttachmentID ids.UUID `json:"attachment_id"`
}

/*
Kind names the job.
*/
func (ScanAttachmentArgs) Kind() string {
	return "scan_attachment"
}

/*
Options keep one waiting scan per file and try it a few times.
*/
func (ScanAttachmentArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: scanAttempts,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRunning, queue.StateScheduled, queue.StateRetryable,
		}},
	}
}

/*
ScanAttachment scans one file and records the verdict.
*/
type ScanAttachment struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	Scanner malware.Scanner
	Storage *filesystem.Manager
}

/*
Handle scans the file unless it was judged already.
*/
func (w *ScanAttachment) Handle(ctx context.Context, job *queue.Job[ScanAttachmentArgs]) error {
	args := job.Args
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	row, err := w.find(ctx, args)
	if err != nil || row.ScanStatus != "pending" {
		return err
	}

	status, err := w.judge(ctx, row)
	if err != nil && job.Attempt < job.MaxAttempts {
		return err
	}

	if err != nil {
		status = scanSkipped
	}

	return w.settle(ctx, row, status)
}

/*
find returns the file, or nothing when it is gone.
*/
func (w *ScanAttachment) find(ctx context.Context, args ScanAttachmentArgs) (models.Attachment, error) {
	var row models.Attachment

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		row, err = models.New(tx).FindAttachment(ctx, models.FindAttachmentParams{
			WorkspaceID: args.WorkspaceID, ID: args.AttachmentID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return models.Attachment{}, nil
	}

	if err != nil {
		return models.Attachment{}, fmt.Errorf("find attachment %s: %w", args.AttachmentID, err)
	}

	return row, nil
}

/*
judge reads the file from its disk and returns the status its verdict
gives it. A file whose bytes are gone or that the scanner could not judge
is skipped; an error means the scan itself failed.
*/
func (w *ScanAttachment) judge(ctx context.Context, row models.Attachment) (string, error) {
	disk, err := w.Storage.Disk(row.Disk)
	if err != nil {
		return "", fmt.Errorf("open disk %s: %w", row.Disk, err)
	}

	body, err := disk.Open(ctx, row.Path)
	if errors.Is(err, filesystem.ErrNotFound) {
		return scanSkipped, nil
	}

	if err != nil {
		return "", fmt.Errorf("open attachment %s: %w", row.ID, err)
	}
	defer func() { _ = body.Close() }()

	verdict, err := w.Scanner.Scan(ctx, malware.File{Path: row.Path, Body: body})
	if err != nil {
		return "", fmt.Errorf("scan attachment %s: %w", row.ID, err)
	}

	switch verdict.Status {
	case malware.Clean:
		return scanClean, nil
	case malware.Infected:
		return scanInfected, nil
	case malware.Unscanned:
		return scanSkipped, nil
	}

	return scanSkipped, nil
}

/*
settle records the status on the file, unless another scan did already,
and tells the viewers of its message, who were shown the file as waiting.
*/
func (w *ScanAttachment) settle(ctx context.Context, row models.Attachment, status string) error {
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		// The message comes from the update, not the row read before the
		// scan: the file may have been put on a message since.
		settled, err := q.SettleAttachmentScan(ctx, models.SettleAttachmentScanParams{
			ScanStatus: status, Now: w.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
		if err != nil || len(settled) == 0 || settled[0] == nil {
			return err
		}

		message := *settled[0]

		conversation, err := q.FindMessageConversation(ctx, models.FindMessageConversationParams{
			WorkspaceID: row.WorkspaceID, ID: message,
		})
		if err != nil {
			return err
		}

		return w.Events.Dispatch(ctx, tx, appevents.MessageUpdated{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: w.Clock.Now(),
				Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
			},
			MessageRef: appevents.MessageRef{
				ConversationRef: appevents.ConversationRef{ConversationID: conversation}, MessageID: message,
			},
		})
	})
	if err != nil {
		return fmt.Errorf("record scan of attachment %s: %w", row.ID, err)
	}

	return nil
}

/*
SweepAttachmentScansArgs names the job; it takes no arguments.
*/
type SweepAttachmentScansArgs struct{}

/*
Kind names the job.
*/
func (SweepAttachmentScansArgs) Kind() string {
	return "sweep_attachment_scans"
}

/*
Options run the sweep once; the next run takes over.
*/
func (SweepAttachmentScansArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: 1}
}

/*
SweepAttachmentScans queues the scan of every file that waited too long,
across every workspace.
*/
type SweepAttachmentScans struct {
	DB    *database.DB
	Queue *queue.Queue
	Clock clock.Clock
}

/*
Handle queues the lost scans.
*/
func (w *SweepAttachmentScans) Handle(ctx context.Context, _ *queue.Job[SweepAttachmentScansArgs]) error {
	waiting, err := models.New(w.DB).ListAttachmentsPendingScan(ctx, models.ListAttachmentsPendingScanParams{
		Cutoff: w.Clock.Now().Add(-scanStaleAfter), RowLimit: scanSweepBatch,
	})
	if err != nil {
		return fmt.Errorf("list attachments pending scan: %w", err)
	}

	for _, file := range waiting {
		err := w.Queue.Push(ctx, ScanAttachmentArgs{WorkspaceID: file.WorkspaceID, AttachmentID: file.ID})
		if err != nil {
			return fmt.Errorf("queue attachment scan: %w", err)
		}
	}

	return nil
}
