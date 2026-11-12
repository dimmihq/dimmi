/*
|--------------------------------------------------------------------------
| Contact Merge Events
|--------------------------------------------------------------------------
|
| Two contacts turned out to be one person and a teammate merged them.
| The event is dispatched in the transaction of the merge, so the audit
| log keeps who did it.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ContactMerged says the source contact was merged into the target, the
contact that survives, and how much moved with it.
*/
type ContactMerged struct {
	Base

	ContactID     ids.UUID
	SourceID      ids.UUID
	Conversations int64
	Identities    int64
}

/*
EventName names the event.
*/
func (ContactMerged) EventName() string { return "contact.merged" }
