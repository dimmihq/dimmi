/*
|--------------------------------------------------------------------------
| Session Revoked
|--------------------------------------------------------------------------
|
| A browser session ended before its time: the user signed out, or
| signed it out from another device. Realtime connections opened with
| that session listen, so they close too.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
SessionRevoked is dispatched once the session row is gone.
*/
type SessionRevoked struct {
	UserID    ids.UUID
	SessionID ids.UUID
}
