/*
|--------------------------------------------------------------------------
| Member Access Changed
|--------------------------------------------------------------------------
|
| Which conversations a member sees changed. Their open connections close
| so every channel is authorized again under the new rule, and the
| workspace's cached list of restricted members is thrown away. The event
| carries both states, so the audit log can say what changed without
| reading anything.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationAccessState is one state of a member's conversation access:
the mode, whether a narrow mode shows conversations nobody holds, and
the teams it names.
*/
type ConversationAccessState struct {
	Mode       string
	Unassigned bool
	TeamIDs    []ids.UUID
}

/*
MemberAccessChanged is dispatched in the transaction that changed the
member's access. ChangedBy is the member who changed it, ids.Nil() when
nobody did.
*/
type MemberAccessChanged struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	From        ConversationAccessState
	To          ConversationAccessState
	ChangedBy   ids.UUID
}
