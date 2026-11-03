/*
|--------------------------------------------------------------------------
| Token Created
|--------------------------------------------------------------------------
|
| A member minted a personal access token. The event names the token and
| what it may do, never its secret, so the audit trail can say who
| handed out which power and when.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TokenCreated is dispatched in the transaction that minted the token.
MemberID and UserID are the member it acts as and their account, and
Abilities what it was granted.
*/
type TokenCreated struct {
	WorkspaceID ids.UUID
	TokenID     ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	Name        string
	Abilities   []string
}
