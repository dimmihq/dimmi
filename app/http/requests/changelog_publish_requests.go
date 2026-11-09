/*
|--------------------------------------------------------------------------
| Changelog Publishing and Label Requests
|--------------------------------------------------------------------------
|
| The inputs of publishing or scheduling an entry, and of a changelog's
| labels.
|
*/

package requests

import "time"

/*
PublishChangelogEntryRequest is the input of publishing an entry at POST
/changelog-entries/{entry}/publications. It publishes now, dated
published_at when given, or holds the entry until scheduled_for; notify
mails the active subscribers once.
*/
type PublishChangelogEntryRequest struct {
	Entry string `path:"entry"`
	Body  struct {
		Notify       bool       `json:"notify,omitempty"`
		PublishedAt  *time.Time `json:"published_at,omitempty"`
		ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	}
}

/*
ChangelogLabelBody is a label's fields.
*/
type ChangelogLabelBody struct {
	Name     *string `json:"name,omitempty"     maxLength:"40"`
	Color    *string `json:"color,omitempty"    maxLength:"7"`
	Position *int32  `json:"position,omitempty"`
}

/*
StoreChangelogLabelRequest is the input of POST
/help-centers/{help_center}/changelog-labels.
*/
type StoreChangelogLabelRequest struct {
	HelpCenter string `path:"help_center"`
	Body       ChangelogLabelBody
}

/*
UpdateChangelogLabelRequest is the input of PATCH
/changelog-labels/{label}.
*/
type UpdateChangelogLabelRequest struct {
	Label string `path:"label"`
	Body  ChangelogLabelBody
}
