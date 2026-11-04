/*
|--------------------------------------------------------------------------
| Token Revoked
|--------------------------------------------------------------------------
|
| A personal access token stopped working before its time, ended by its
| own member or by an admin who manages every member's tokens.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TokenRevoked is dispatched in the transaction that revoked the token.
MemberID is the member the token acted as, UserID their account and
RevokedBy the member who ended it.
*/
type TokenRevoked struct {
	WorkspaceID ids.UUID
	TokenID     ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	RevokedBy   ids.UUID
}
