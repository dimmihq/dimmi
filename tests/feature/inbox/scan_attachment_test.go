package inbox_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/malware"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
errScannerDown is what a scanner that cannot be reached answers.
*/
var errScannerDown = errors.New("scanner down")

/*
downScanner never reaches its scanner.
*/
type downScanner struct{}

/*
Scan always fails.
*/
func (downScanner) Scan(context.Context, malware.File) (malware.Verdict, error) {
	return malware.Verdict{}, errScannerDown
}

/*
stored writes the contents on the file's disk, under the file's path.
*/
func stored(t *testing.T, tc *tests.TestCase, file models.Attachment, contents string) {
	t.Helper()

	disk, err := tc.App.Storage.Disk(file.Disk)
	require.NoError(t, err)
	require.NoError(t, disk.Put(t.Context(), file.Path, []byte(contents), filesystem.PutOptions{}))
}

/*
scanJob returns the job that scans files with the scanner.
*/
func scanJob(tc *tests.TestCase, scanner malware.Scanner) *jobs.ScanAttachment {
	return &jobs.ScanAttachment{
		DB: tc.App.DB, Clock: tc.Clock, Events: tc.App.Events, Scanner: scanner, Storage: tc.App.Storage,
	}
}

/*
scanAttempt runs the given attempt, out of five, of the file's scan.
*/
func scanAttempt(job *jobs.ScanAttachment, file models.Attachment, attempt int) error {
	return job.Handle(context.Background(), &queue.Job[jobs.ScanAttachmentArgs]{
		Args:    jobs.ScanAttachmentArgs{WorkspaceID: file.WorkspaceID, AttachmentID: file.ID},
		Attempt: attempt, MaxAttempts: 5,
	})
}

/*
scan runs the first attempt of the file's scan.
*/
func scan(job *jobs.ScanAttachment, file models.Attachment) error {
	return scanAttempt(job, file, 1)
}

/*
statusOf returns the scan status the file has now.
*/
func statusOf(t *testing.T, tc *tests.TestCase, file models.Attachment) string {
	t.Helper()

	workspace := file.WorkspaceID
	found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
		return q.FindAttachment(ctx, models.FindAttachmentParams{WorkspaceID: workspace, ID: file.ID})
	})
	require.NoError(t, err)

	return found.ScanStatus
}

func TestScanAttachment(t *testing.T) {
	t.Parallel()

	t.Run("releases a clean file and blocks an infected one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		clean, infected := upload(t, tc, workspace, "notes.pdf"), upload(t, tc, workspace, "invoice.pdf")
		stored(t, tc, clean, "hello there")
		stored(t, tc, infected, malware.EICAR())
		job := scanJob(tc, tc.App.Scanner.Fake())

		require.NoError(t, scan(job, clean))
		require.NoError(t, scan(job, infected))

		assert.Equal(t, "clean", statusOf(t, tc, clean))
		assert.Equal(t, "infected", statusOf(t, tc, infected))
	})

	t.Run("releases a file the scanner could not judge or whose bytes are gone as not scanned", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		big, gone := upload(t, tc, workspace, "big.pdf"), upload(t, tc, workspace, "gone.pdf")
		stored(t, tc, big, strings.Repeat("a", 64))
		job := scanJob(tc, tc.App.Scanner.Fake().Set(big.Path, malware.Verdict{Status: malware.Unscanned}))

		require.NoError(t, scan(job, big))
		require.NoError(t, scan(job, gone))

		assert.Equal(t, "skipped", statusOf(t, tc, big))
		assert.Equal(t, "skipped", statusOf(t, tc, gone))
	})

	t.Run("tries again while the scanner is down and releases the file at the last attempt", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		file := upload(t, tc, tc.Workspace.Workspace.ID, "notes.pdf")
		stored(t, tc, file, "hello there")
		job := scanJob(tc, downScanner{})

		require.ErrorIs(t, scan(job, file), errScannerDown)
		assert.Equal(t, "pending", statusOf(t, tc, file))

		require.NoError(t, scanAttempt(job, file, 5))
		assert.Equal(t, "skipped", statusOf(t, tc, file))
	})

	t.Run("leaves a file already judged, or deleted, alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		judged := upload(t, tc, workspace, "notes.pdf")
		stored(t, tc, judged, malware.EICAR())
		_, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Attachment, error) {
			return q.SetAttachmentScanStatus(ctx, models.SetAttachmentScanStatusParams{
				WorkspaceID: workspace, ID: judged.ID, ScanStatus: "clean", Now: tc.Clock.Now(),
			})
		})
		require.NoError(t, err)
		job := scanJob(tc, tc.App.Scanner.Fake())

		require.NoError(t, scan(job, judged))
		assert.Equal(t, "clean", statusOf(t, tc, judged))

		missing := judged
		missing.ID = ids.New()
		require.NoError(t, scan(job, missing))
	})

	t.Run("sweeps files that waited too long into a scan, not new or unfinished ones", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.App.Scanner.Fake()
		workspace := tc.Workspace.Workspace.ID
		lost, fresh, unfinished := upload(t, tc, workspace, "lost.pdf"), upload(t, tc, workspace, "fresh.pdf"),
			upload(t, tc, workspace, "unfinished.pdf")
		for _, file := range []models.Attachment{lost, fresh, unfinished} {
			stored(t, tc, file, "hello there")
		}

		_, err := tc.Operator().Exec(t.Context(),
			"UPDATE attachments SET uploaded_at = $2 WHERE id = ANY($1)", []ids.UUID{lost.ID, fresh.ID}, tc.Clock.Now())
		require.NoError(t, err)
		tc.Clock.Travel(10 * time.Minute)
		_, err = tc.Operator().Exec(t.Context(),
			"UPDATE attachments SET updated_at = $2 WHERE id = $1", fresh.ID, tc.Clock.Now())
		require.NoError(t, err)
		sweep := &jobs.SweepAttachmentScans{DB: tc.App.DB, Queue: tc.App.Queue, Clock: tc.Clock}

		require.NoError(t, sweep.Handle(t.Context(), &queue.Job[jobs.SweepAttachmentScansArgs]{}))
		tc.Drain()

		assert.Equal(t, "clean", statusOf(t, tc, lost))
		assert.Equal(t, "pending", statusOf(t, tc, fresh))
		assert.Equal(t, "pending", statusOf(t, tc, unfinished))
	})
}
