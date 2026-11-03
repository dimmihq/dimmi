/*
|--------------------------------------------------------------------------
| User Email Verified
|--------------------------------------------------------------------------
|
| A user just proved they read mail at their address, which unlocks the
| steps that reach other people, such as inviting them.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
UserEmailVerified is dispatched in the transaction that verified the
user's email.
*/
type UserEmailVerified struct {
	UserID ids.UUID
}
