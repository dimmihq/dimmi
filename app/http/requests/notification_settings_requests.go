/*
|--------------------------------------------------------------------------
| Notification Settings And Push Requests
|--------------------------------------------------------------------------
|
| A member's notification settings, where fields left out stay as they
| are, and a browser's push subscription as the browser gives it.
|
*/

package requests

import "time"

/*
UpdateNotificationSettingsBody changes a member's settings. A null
paused_until with clear_pause ends a pause.
*/
type UpdateNotificationSettingsBody struct {
	Timezone          *string    `json:"timezone,omitempty"            maxLength:"64"`
	QuietHoursEnabled *bool      `json:"quiet_hours_enabled,omitempty"`
	QuietHoursStart   *string    `json:"quiet_hours_start,omitempty"   maxLength:"5"`
	QuietHoursEnd     *string    `json:"quiet_hours_end,omitempty"     maxLength:"5"`
	QuietHoursDays    *int16     `json:"quiet_hours_days,omitempty"`
	QuietHoursBypass  *[]string  `json:"quiet_hours_bypass,omitempty"  maxItems:"20"`
	PausedUntil       *time.Time `json:"paused_until,omitempty"`
	ClearPause        bool       `json:"clear_pause,omitempty"`
	ShowPreviews      *bool      `json:"show_previews,omitempty"`
	IncludeAIHandled  *bool      `json:"include_ai_handled,omitempty"`
}

/*
UpdateNotificationSettingsRequest is the input of PATCH
/notification-settings.
*/
type UpdateNotificationSettingsRequest struct {
	Body UpdateNotificationSettingsBody
}

/*
PushKeysBody are a subscription's keys, base64url.
*/
type PushKeysBody struct {
	P256dh string `json:"p256dh" maxLength:"200"`
	Auth   string `json:"auth"   maxLength:"100"`
}

/*
StorePushSubscriptionRequest is the input of POST /push-subscriptions.
*/
type StorePushSubscriptionRequest struct {
	Body struct {
		Endpoint    string       `json:"endpoint"               maxLength:"2048"`
		Keys        PushKeysBody `json:"keys"`
		DeviceLabel string       `json:"device_label,omitempty" maxLength:"80"`
	}
}

/*
PushSubscriptionRequest names a push subscription in the path.
*/
type PushSubscriptionRequest struct {
	Subscription string `path:"subscription"`
}
