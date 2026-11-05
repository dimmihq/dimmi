/*
|--------------------------------------------------------------------------
| Workspace Access Policy Changed
|--------------------------------------------------------------------------
|
| A workspace's rules for who may work in it changed. Open connections of
| its members close at once, so a tightened rule bites on the very next
| thing they do, and the change lands in the audit log with its values.
| The values never leave the server over realtime: a list of a company's
| office addresses is nobody's live update.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WorkspaceAccessPolicyChanged is dispatched in the transaction that
changed the rules. ChangedBy is the member who did it, ids.Nil() for an
operator at the server. Changes holds, per setting, what it was and what
it became.
*/
type WorkspaceAccessPolicyChanged struct {
	WorkspaceID ids.UUID
	ChangedBy   ids.UUID
	Changes     map[string]any
}
