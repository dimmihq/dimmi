/*
|--------------------------------------------------------------------------
| Catalog: Notification Deliveries
|--------------------------------------------------------------------------
|
| The delivery log: how each notification reached a teammate on each
| channel, or why it did not.
|
*/

package mcpserver

/*
notificationDeliveryDocs returns the entries of the delivery log.
*/
func notificationDeliveryDocs() map[string]Doc {
	return map[string]Doc{
		"notification-deliveries.index": {
			Title: "List notification deliveries",
			Use: "Shows how your notifications reached you on each channel (in the app, email, push, Slack): " +
				"sent, failed and why, or skipped and why, such as quiet hours or a channel turned off. " +
				"Someone who manages members may read another member's.",
			Returns: "a page of deliveries, newest first, with next_cursor in meta.",
			Next:    []string{"dimmi_notification_preferences_update: turn a channel back on"},
			Params: map[string]string{
				"filter[member]":  "A member id, mem_..., to read their log; needs members:write. Yours by default.",
				"filter[channel]": "in_app, email, push or slack.",
				"filter[status]":  "sent, failed or skipped.",
				"limit":           "How many to return, up to 100. 25 by default.",
			},
		},
		"notifications.deliveries.index": {
			Title:   "Show how a notification reached you",
			Use:     "Lists each channel one of your own notifications went to, and how it ended.",
			Returns: "the deliveries of the notification, oldest first.",
		},
	}
}
