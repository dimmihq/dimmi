/*
|--------------------------------------------------------------------------
| Email Classification
|--------------------------------------------------------------------------
|
| What kind of mail arrived decides what it may cause. Only a person's
| own words open conversations and wake teammates; an out-of-office
| reply, a bounce or a newsletter is kept for the record but never
| answered, so two automated senders can never keep each other busy.
|
*/

package enums

/*
EmailClassification is the kind of an inbound email.
*/
type EmailClassification string

/*
The classifications, in the order of the database check.
*/
const (
	EmailClassificationNormal       EmailClassification = "normal"
	EmailClassificationAutoReply    EmailClassification = "auto_reply"
	EmailClassificationBounce       EmailClassification = "bounce"
	EmailClassificationComplaint    EmailClassification = "complaint"
	EmailClassificationList         EmailClassification = "list"
	EmailClassificationBulk         EmailClassification = "bulk"
	EmailClassificationNoreply      EmailClassification = "noreply"
	EmailClassificationSystemNotice EmailClassification = "system_notice"
	EmailClassificationLoop         EmailClassification = "loop"
	EmailClassificationSpam         EmailClassification = "spam"
	EmailClassificationForwarded    EmailClassification = "forwarded"
	EmailClassificationTest         EmailClassification = "test"
)

/*
EmailClassifications returns every classification, in the order of the
database check.
*/
func EmailClassifications() []EmailClassification {
	return []EmailClassification{
		EmailClassificationNormal, EmailClassificationAutoReply, EmailClassificationBounce,
		EmailClassificationComplaint, EmailClassificationList, EmailClassificationBulk, EmailClassificationNoreply,
		EmailClassificationSystemNotice, EmailClassificationLoop, EmailClassificationSpam,
		EmailClassificationForwarded, EmailClassificationTest,
	}
}
