/*
|--------------------------------------------------------------------------
| Notification Requests
|--------------------------------------------------------------------------
|
| The inputs of a member's own notification endpoints: the bell's list
| and its read state, the settings matrix and settings, the focus
| heartbeat and test sends. Each acts on the calling member only.
|
*/

package requests

/*
IndexNotificationsRequest filters the list: unread only, one trigger,
and the page after a cursor.
*/
type IndexNotificationsRequest struct {
	Status  string `query:"filter[status]"  enum:"all,unread,"`
	Trigger string `query:"filter[trigger]" maxLength:"40"`
	Cursor  string `query:"cursor"          maxLength:"80"`
	Limit   int32  `query:"limit"           minimum:"0" maximum:"100"`
}

/*
UpdateNotificationsBody marks notifications: the ids named, or every
unread one when none are, read or unread; or the badge seen.
*/
type UpdateNotificationsBody struct {
	IDs  []string `json:"ids,omitempty"  maxItems:"100"`
	Read *bool    `json:"read,omitempty"`
	Seen *bool    `json:"seen,omitempty"`
}

/*
UpdateNotificationsRequest is the input of PATCH /notifications.
*/
type UpdateNotificationsRequest struct {
	Body UpdateNotificationsBody
}
