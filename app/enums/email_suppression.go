/*
|--------------------------------------------------------------------------
| Email Suppression
|--------------------------------------------------------------------------
|
| An address stops receiving mail for a reason and within a scope. A
| hard bounce or a complaint about support mail stops everything; an
| unsubscribe or a complaint about one kind of mail stops only that
| kind, so the customer's own conversations still reach them.
|
*/

package enums

/*
EmailSuppressionReason is why an address is not mailed.
*/
type EmailSuppressionReason string

/*
The reasons, in the order of the database check.
*/
const (
	EmailSuppressionReasonHardBounce   EmailSuppressionReason = "hard_bounce"
	EmailSuppressionReasonSoftBounce   EmailSuppressionReason = "soft_bounce"
	EmailSuppressionReasonComplaint    EmailSuppressionReason = "complaint"
	EmailSuppressionReasonUnsubscribed EmailSuppressionReason = "unsubscribed"
	EmailSuppressionReasonManual       EmailSuppressionReason = "manual"
	EmailSuppressionReasonInvalid      EmailSuppressionReason = "invalid"
)

/*
EmailSuppressionReasons returns every reason, in the order of the
database check.
*/
func EmailSuppressionReasons() []EmailSuppressionReason {
	return []EmailSuppressionReason{
		EmailSuppressionReasonHardBounce, EmailSuppressionReasonSoftBounce, EmailSuppressionReasonComplaint,
		EmailSuppressionReasonUnsubscribed, EmailSuppressionReasonManual, EmailSuppressionReasonInvalid,
	}
}

/*
EmailSuppressionScope is which mail a suppression stops.
*/
type EmailSuppressionScope string

/*
The scopes, in the order of the database check.
*/
const (
	EmailSuppressionScopeAll           EmailSuppressionScope = "all"
	EmailSuppressionScopeNotifications EmailSuppressionScope = "notifications"
	EmailSuppressionScopeChangelog     EmailSuppressionScope = "changelog"
	EmailSuppressionScopeFeedback      EmailSuppressionScope = "feedback"
)

/*
EmailSuppressionScopes returns every scope, in the order of the
database check.
*/
func EmailSuppressionScopes() []EmailSuppressionScope {
	return []EmailSuppressionScope{
		EmailSuppressionScopeAll, EmailSuppressionScopeNotifications, EmailSuppressionScopeChangelog,
		EmailSuppressionScopeFeedback,
	}
}
