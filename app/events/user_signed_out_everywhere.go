/*
|--------------------------------------------------------------------------
| User Signed Out Everywhere
|--------------------------------------------------------------------------
|
| Everything that let a user in ended at once: a password reset, or the
| owner of an inbox taking back an account a stranger registered with
| it. Realtime connections opened with those credentials listen, so
| they close too.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
UserSignedOutEverywhere is dispatched in the transaction that ended the
user's sessions, app grants and access tokens.
*/
type UserSignedOutEverywhere struct {
	UserID ids.UUID
}
