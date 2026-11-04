/*
|--------------------------------------------------------------------------
| User Deleted
|--------------------------------------------------------------------------
|
| A person deleted their account. Everything that let them in ended, and
| what remains of them is a nameless row that keeps their old messages
| attributed. Connections still open close, and anything else holding on
| to the person lets go when it hears this.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
UserDeleted is dispatched in the transaction that anonymized the user.
*/
type UserDeleted struct {
	UserID ids.UUID
}
