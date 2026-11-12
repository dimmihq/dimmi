/*
|--------------------------------------------------------------------------
| Outbound Email
|--------------------------------------------------------------------------
|
| Every email a workspace sends is one kind of mail, a reply above all,
| and moves from queued to sent and on to delivered, or ends bounced,
| complained about, failed or suppressed before it ever left.
|
*/

package enums

/*
OutboundEmailKind is why an email was sent.
*/
type OutboundEmailKind string

/*
The kinds, in the order of the database check.
*/
const (
	OutboundEmailKindReply           OutboundEmailKind = "reply"
	OutboundEmailKindAIReply         OutboundEmailKind = "ai_reply"
	OutboundEmailKindSystem          OutboundEmailKind = "system"
	OutboundEmailKindWidgetOffline   OutboundEmailKind = "widget_offline"
	OutboundEmailKindNewConversation OutboundEmailKind = "new_conversation"
	OutboundEmailKindTest            OutboundEmailKind = "test"
	OutboundEmailKindChangelog       OutboundEmailKind = "changelog"
	OutboundEmailKindFeedbackUpdate  OutboundEmailKind = "feedback_update"
	OutboundEmailKindPortalSignIn    OutboundEmailKind = "portal_sign_in"
	OutboundEmailKindBroadcast       OutboundEmailKind = "broadcast"
	OutboundEmailKindRating          OutboundEmailKind = "conversation_rating"
)

/*
OutboundEmailKinds returns every kind, in the order of the database
check.
*/
func OutboundEmailKinds() []OutboundEmailKind {
	return []OutboundEmailKind{
		OutboundEmailKindReply, OutboundEmailKindAIReply, OutboundEmailKindSystem, OutboundEmailKindWidgetOffline,
		OutboundEmailKindNewConversation, OutboundEmailKindTest, OutboundEmailKindChangelog,
		OutboundEmailKindFeedbackUpdate, OutboundEmailKindPortalSignIn, OutboundEmailKindBroadcast,
		OutboundEmailKindRating,
	}
}

/*
OutboundEmailStatus is where a sent email stands.
*/
type OutboundEmailStatus string

/*
The statuses, in the order of the database check.
*/
const (
	OutboundEmailStatusQueued     OutboundEmailStatus = "queued"
	OutboundEmailStatusSent       OutboundEmailStatus = "sent"
	OutboundEmailStatusDelivered  OutboundEmailStatus = "delivered"
	OutboundEmailStatusBounced    OutboundEmailStatus = "bounced"
	OutboundEmailStatusComplained OutboundEmailStatus = "complained"
	OutboundEmailStatusFailed     OutboundEmailStatus = "failed"
	OutboundEmailStatusSuppressed OutboundEmailStatus = "suppressed"
)

/*
OutboundEmailStatuses returns every status, in the order of the
database check.
*/
func OutboundEmailStatuses() []OutboundEmailStatus {
	return []OutboundEmailStatus{
		OutboundEmailStatusQueued, OutboundEmailStatusSent, OutboundEmailStatusDelivered,
		OutboundEmailStatusBounced, OutboundEmailStatusComplained, OutboundEmailStatusFailed,
		OutboundEmailStatusSuppressed,
	}
}
