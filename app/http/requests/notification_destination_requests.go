/*
|--------------------------------------------------------------------------
| Notification Destination Requests
|--------------------------------------------------------------------------
|
| A Slack channel the team hears in, or a teammate's own direct message,
| named by public ids. Fields a change leaves out stay as they are; an
| inbox list left out means every inbox.
|
*/

package requests

/*
NotificationDestinationRequest names a destination in the path.
*/
type NotificationDestinationRequest struct {
	Destination string `path:"destination"`
}

/*
StoreNotificationDestinationBody makes a destination: a slack_channel
of the workspace posting in the channel, or the caller's slack_dm.
*/
type StoreNotificationDestinationBody struct {
	Type             string    `json:"type"                         validate:"required,oneof=slack_channel slack_dm"`
	IntegrationID    string    `json:"integration_id"               validate:"required,max=80"`
	ChannelID        string    `json:"channel_id,omitempty"         validate:"max=40"`
	Name             string    `json:"name,omitempty"               validate:"max=80"`
	Triggers         []string  `json:"triggers,omitempty"           validate:"max=10"`
	InboxIDs         *[]string `json:"inbox_ids,omitempty"          validate:"omitempty,max=50"`
	IncludeAIHandled bool      `json:"include_ai_handled,omitempty"`
	ShowPreviews     *bool     `json:"show_previews,omitempty"`
	Digests          []string  `json:"digests,omitempty"            validate:"max=3"`
	DigestTime       string    `json:"digest_time,omitempty"        validate:"max=5"`
}

/*
StoreNotificationDestinationRequest is the input of POST
/notification-destinations.
*/
type StoreNotificationDestinationRequest struct {
	Body StoreNotificationDestinationBody
}

/*
UpdateNotificationDestinationBody changes what a destination hears
about; enabled turns it off, or back on after an error.
*/
type UpdateNotificationDestinationBody struct {
	Name             *string   `json:"name,omitempty"               validate:"omitempty,min=1,max=80"`
	Triggers         *[]string `json:"triggers,omitempty"           validate:"omitempty,max=10"`
	InboxIDs         *[]string `json:"inbox_ids,omitempty"          validate:"omitempty,max=50"`
	AllInboxes       bool      `json:"all_inboxes,omitempty"`
	IncludeAIHandled *bool     `json:"include_ai_handled,omitempty"`
	ShowPreviews     *bool     `json:"show_previews,omitempty"`
	Digests          *[]string `json:"digests,omitempty"            validate:"omitempty,max=3"`
	DigestTime       *string   `json:"digest_time,omitempty"        validate:"omitempty,max=5"`
	Enabled          *bool     `json:"enabled,omitempty"`
}

/*
UpdateNotificationDestinationRequest is the input of PATCH
/notification-destinations/{destination}.
*/
type UpdateNotificationDestinationRequest struct {
	Destination string `path:"destination"`
	Body        UpdateNotificationDestinationBody
}
