/*
|--------------------------------------------------------------------------
| Notification Delivery Requests
|--------------------------------------------------------------------------
|
| The inputs of the delivery log: the caller's own lines, or another
| member's for someone who manages members, narrowed by channel and
| status; the lines of one bell row; and the signed unsubscribe links
| in teammate emails.
|
*/

package requests

/*
IndexNotificationDeliveriesRequest filters the log.
*/
type IndexNotificationDeliveriesRequest struct {
	Member  string `query:"filter[member]"  maxLength:"40"`
	Channel string `query:"filter[channel]" enum:"in_app,email,push,slack,"`
	Status  string `query:"filter[status]"  enum:"sent,failed,skipped,"`
	Cursor  string `query:"cursor"          maxLength:"80"`
	Limit   int32  `query:"limit"           minimum:"0" maximum:"100"`
}

/*
NotificationDeliveriesOfRequest names one bell row.
*/
type NotificationDeliveriesOfRequest struct {
	Notification string `path:"notification"`
}

/*
NotificationUnsubscribeRequest names an unsubscribe link's token.
*/
type NotificationUnsubscribeRequest struct {
	Token string `path:"token" maxLength:"200"`
}
