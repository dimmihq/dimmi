/*
|--------------------------------------------------------------------------
| Attachment Resources
|--------------------------------------------------------------------------
|
| A file on a message as a teammate sees it, with a short-lived link
| once the malware scan lets it through.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Attachment is a file on a message as a teammate sees it. URL reads it
for a few minutes and is null until the malware scan lets it through.
Blocked marks a file the scan refused. Scanned marks one the scan found
clean, so a file with a link that is not Scanned was never checked.
ContentID names a picture an email's text shows in place, as
cid:<content id>.
*/
type Attachment struct {
	ID          string  `json:"id"`
	Object      string  `json:"object"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	SizeBytes   int64   `json:"size_bytes"`
	Width       *int32  `json:"width"`
	Height      *int32  `json:"height"`
	URL         *string `json:"url"`
	Blocked     bool    `json:"blocked"`
	Scanned     bool    `json:"scanned"`
	ContentID   *string `json:"content_id"`
}

/*
NewAttachment shapes a file of a message.
*/
func NewAttachment(f messages.File) Attachment {
	return Attachment{
		ID: models.PublicID(models.PrefixAttachment, f.Row.ID), Object: "attachment", Filename: f.Row.Filename,
		ContentType: f.Row.ContentType, SizeBytes: f.Row.SizeBytes, Width: f.Row.Width, Height: f.Row.Height,
		URL: f.URL, Blocked: f.Blocked, Scanned: f.Row.ScanStatus == "clean", ContentID: f.Row.ContentID,
	}
}
