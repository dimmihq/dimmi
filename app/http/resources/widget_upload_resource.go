/*
|--------------------------------------------------------------------------
| Widget Upload Resource
|--------------------------------------------------------------------------
|
| Tells a visitor's messenger where to send a file's bytes. The address is
| either signed storage, which takes the bytes as they are, or this
| install's own upload route, which takes them with the session's bearer.
|
*/

package resources

import (
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WidgetUploadTarget is where and how to send the bytes.
*/
type WidgetUploadTarget struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

/*
WidgetUpload is a pending upload with where its bytes go.
*/
type WidgetUpload struct {
	ID     string             `json:"id"`
	Object string             `json:"object"`
	Status string             `json:"status"`
	Upload WidgetUploadTarget `json:"upload"`
	Links  map[string]string  `json:"links"`
}

/*
NewWidgetUpload presents a pending upload. Base is the address of this
install's widget API, which takes direct uploads.
*/
func NewWidgetUpload(p attachments.PendingUpload, base string) WidgetUpload {
	id := models.PublicID(models.PrefixAttachment, p.Attachment.ID)
	self := base + "/attachments/" + id
	target := WidgetUploadTarget{
		Method: "PUT", URL: p.URL, ExpiresAt: p.ExpiresAt.UTC(),
		Headers: map[string]string{
			"Content-Type":   p.Attachment.ContentType,
			"Content-Length": strconv.FormatInt(p.Attachment.SizeBytes, 10),
		},
	}

	if p.Direct {
		target.URL = self + "/content"
	}

	return WidgetUpload{
		ID: id, Object: "attachment_upload", Status: "pending", Upload: target, Links: map[string]string{"self": self},
	}
}
