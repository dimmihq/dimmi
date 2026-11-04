/*
|--------------------------------------------------------------------------
| First-Party App Grants
|--------------------------------------------------------------------------
|
| The user's own installed apps hold grants that act like browser
| sessions. Whatever ends every browser session for safety, such as a
| password reset or a defended account takeover, must end those grants
| in the same breath, or a stolen phone would outlive the reset. Apps
| sign in through the OAuth server, which owns the grants; until it is
| installed there are none, and nothing needs ending.
|
*/

package auth

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The reasons first-party grants are revoked for, which the app shows the
user when it is signed out.
*/
const (
	RevokedForPasswordReset    = "password_reset"
	RevokedForPreHijackDefense = "pre_hijack_defense"
	RevokedForPasswordChanged  = "password_changed"
	RevokedForUserDeleted      = "user_deleted"
)

/*
AppGrants revokes the user's first-party app grants.
*/
type AppGrants interface {
	/*
		RevokeUserAppGrants revokes every first-party grant of the user
		but the kept one, ids.Nil() keeping none, for the reason.
	*/
	RevokeUserAppGrants(ctx context.Context, userID ids.UUID, reason string, keep ids.UUID) error
}

/*
NoAppGrants is the grant store of an install without first-party apps:
there is never a grant to revoke.
*/
type NoAppGrants struct{}

/*
RevokeUserAppGrants has nothing to revoke.
*/
func (NoAppGrants) RevokeUserAppGrants(context.Context, ids.UUID, string, ids.UUID) error {
	return nil
}
