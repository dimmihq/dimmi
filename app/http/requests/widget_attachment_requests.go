/*
|--------------------------------------------------------------------------
| Widget Attachment Requests
|--------------------------------------------------------------------------
|
| What a visitor's messenger sends to upload a file: the file it is about
| to send, then that its bytes arrived.
|
*/

package requests

/*
StoreWidgetAttachmentRequest is the input of POST /attachments: the file's
name, type and exact size in bytes.
*/
type StoreWidgetAttachmentRequest struct {
	Body struct {
		_           struct{} `additionalProperties:"true"`
		Filename    string   `json:"filename"     maxLength:"255" minLength:"1"`
		ContentType string   `json:"content_type" maxLength:"255" minLength:"1"`
		SizeBytes   int64    `json:"size_bytes"   minimum:"1"`
	}
}

/*
UpdateWidgetAttachmentRequest is the input of PATCH /attachments/{id}.
Status names the state asked for; only "uploaded" is a step an upload
can take.
*/
type UpdateWidgetAttachmentRequest struct {
	Attachment string `path:"attachment"`
	Body       struct {
		_      struct{} `additionalProperties:"true"`
		Status string   `json:"status" maxLength:"32"`
	}
}
