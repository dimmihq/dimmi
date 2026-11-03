/*
|--------------------------------------------------------------------------
| Conversation Status
|--------------------------------------------------------------------------
|
| A conversation is open while it needs the team, snoozed until a set
| time, and closed once it is done. Spam is not a status: it is a flag
| beside it, so taking a conversation out of spam restores its status.
|
*/

package enums

/*
ConversationStatus is where a conversation stands.
*/
type ConversationStatus string

/*
The conversation statuses.
*/
const (
	ConversationStatusOpen    ConversationStatus = "open"
	ConversationStatusSnoozed ConversationStatus = "snoozed"
	ConversationStatusClosed  ConversationStatus = "closed"
)

/*
ConversationStatuses returns every status, in the order of the database
check.
*/
func ConversationStatuses() []ConversationStatus {
	return []ConversationStatus{ConversationStatusOpen, ConversationStatusSnoozed, ConversationStatusClosed}
}

/*
Valid reports whether the status is one of the statuses.
*/
func (s ConversationStatus) Valid() bool {
	switch s {
	case ConversationStatusOpen, ConversationStatusSnoozed, ConversationStatusClosed:
		return true
	}

	return false
}
