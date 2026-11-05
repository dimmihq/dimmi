/*
|--------------------------------------------------------------------------
| Message Files
|--------------------------------------------------------------------------
|
| The files a page of a timeline carries, as a teammate sees them. A file
| the malware scan cleared, or did not need to check, gets a link that
| reads it for a few minutes, signed by the disk it lives on; one the scan
| refused is marked and never linked; one still waiting on the scan shows
| without a link until it is cleared.
|
*/

package messages

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
fileLinkTTL is how long a link to a file reads it.
*/
const fileLinkTTL = 5 * time.Minute

/*
Disks finds the disk a file lives on by its name.
*/
type Disks interface {
	/*
		Disk returns the disk with the name.
	*/
	Disk(name string) (filesystem.Filesystem, error)
}

/*
File is a message's file with its link, nil until it may be read, and
whether the scan refused it.
*/
type File struct {
	Row     models.Attachment
	URL     *string
	Blocked bool
}

/*
loadFiles reads the files of the messages, by message.
*/
func loadFiles(ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.Message) (
	map[ids.UUID][]models.Attachment, error,
) {
	if len(rows) == 0 {
		return nil, nil
	}

	messageIDs := make([]ids.UUID, 0, len(rows))
	for _, m := range rows {
		messageIDs = append(messageIDs, m.ID)
	}

	files, err := q.ListMessageAttachments(ctx, models.ListMessageAttachmentsParams{
		WorkspaceID: ws, MessageIds: messageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list message files: %w", err)
	}

	out := make(map[ids.UUID][]models.Attachment, len(files))
	for _, f := range files {
		if f.MessageID != nil {
			out[*f.MessageID] = append(out[*f.MessageID], f)
		}
	}

	return out, nil
}

/*
presentFiles links each file the scan let through.
*/
func presentFiles(ctx context.Context, disks Disks, rows map[ids.UUID][]models.Attachment) map[ids.UUID][]File {
	out := make(map[ids.UUID][]File, len(rows))
	for message, files := range rows {
		for _, row := range files {
			out[message] = append(out[message], PresentFile(ctx, disks, row))
		}
	}

	return out
}

/*
PresentFile links a file the scan let through and marks one it refused.
*/
func PresentFile(ctx context.Context, disks Disks, row models.Attachment) File {
	file := File{Row: row, Blocked: row.ScanStatus == "infected"}
	if disks == nil || (row.ScanStatus != "clean" && row.ScanStatus != "skipped") {
		return file
	}

	disk, err := disks.Disk(row.Disk)
	if err != nil {
		return file
	}

	if url, err := disk.TemporaryURL(ctx, row.Path, fileLinkTTL); err == nil {
		file.URL = &url
	}

	return file
}
