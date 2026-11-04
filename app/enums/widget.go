/*
|--------------------------------------------------------------------------
| Widget Values
|--------------------------------------------------------------------------
|
| The values a messenger widget's settings and sessions take, each matching
| its database check.
|
*/

package enums

/*
WidgetSessionKind is how far a visitor is known: anonymous, or identified
by a verified identity token.
*/
type WidgetSessionKind string

/*
The session kinds.
*/
const (
	WidgetSessionAnonymous  WidgetSessionKind = "anonymous"
	WidgetSessionIdentified WidgetSessionKind = "identified"
)

/*
WidgetCollectEmail says when the widget asks a lead for an email.
*/
type WidgetCollectEmail string

/*
The times the widget asks.
*/
const (
	WidgetCollectNever   WidgetCollectEmail = "never"
	WidgetCollectOffline WidgetCollectEmail = "offline"
	WidgetCollectAlways  WidgetCollectEmail = "always"
)

/*
WidgetReplyTime is the reply time a widget promises; auto follows team
availability.
*/
type WidgetReplyTime string

/*
The reply times.
*/
const (
	WidgetReplyAuto    WidgetReplyTime = "auto"
	WidgetReplyMinutes WidgetReplyTime = "minutes"
	WidgetReplyHours   WidgetReplyTime = "hours"
	WidgetReplyDay     WidgetReplyTime = "day"
)

/*
WidgetRevokeReason says why a session ended.
*/
type WidgetRevokeReason string

/*
The reasons a session ends.
*/
const (
	WidgetRevokeLogout         WidgetRevokeReason = "logout"
	WidgetRevokeIdentitySwitch WidgetRevokeReason = "identity_switch"
	WidgetRevokeContactMerged  WidgetRevokeReason = "contact_merged"
	WidgetRevokeContactErased  WidgetRevokeReason = "contact_erased"
	WidgetRevokeSecretRevoked  WidgetRevokeReason = "secret_revoked"
	WidgetRevokeExpired        WidgetRevokeReason = "expired"
	WidgetRevokeWidgetDeleted  WidgetRevokeReason = "widget_deleted"
	WidgetRevokeBot            WidgetRevokeReason = "bot"
	WidgetRevokeAdmin          WidgetRevokeReason = "admin"
)
