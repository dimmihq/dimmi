/*
|--------------------------------------------------------------------------
| Notification Preference Requests
|--------------------------------------------------------------------------
|
| Changes to a member's settings matrix, the dashboard's focus heartbeat
| and test sends.
|
*/

package requests

/*
PreferenceCellBody turns one trigger on one channel on or off.
*/
type PreferenceCellBody struct {
	Trigger string `json:"trigger" maxLength:"40"`
	Channel string `json:"channel" enum:"in_app,email,push"`
	Enabled bool   `json:"enabled"`
}

/*
UpdateNotificationPreferencesRequest is the input of PATCH
/notification-preferences.
*/
type UpdateNotificationPreferencesRequest struct {
	Body struct {
		Preferences []PreferenceCellBody `json:"preferences" maxItems:"50"`
	}
}

/*
NotificationFocusRequest is the dashboard's heartbeat: whether it is
visible and the conversation it shows.
*/
type NotificationFocusRequest struct {
	Body struct {
		Visible        bool   `json:"visible"`
		ConversationID string `json:"conversation_id,omitempty" maxLength:"64"`
	}
}

/*
NotificationTestRequest names the channel to test.
*/
type NotificationTestRequest struct {
	Body struct {
		Channel string `json:"channel" enum:"in_app,email,push"`
	}
}
