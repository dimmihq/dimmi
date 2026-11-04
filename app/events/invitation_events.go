/*
|--------------------------------------------------------------------------
| Invitation Events
|--------------------------------------------------------------------------
|
| An invitation was sent, accepted or withdrawn. Admins watching the
| workspace see the list of pending invitations change; the person
| invited joins the workspace through MemberJoined, not through these.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
InvitationCreated is dispatched in the transaction that created the
invitation. InvitedBy is the member who sent it.
*/
type InvitationCreated struct {
	WorkspaceID  ids.UUID
	InvitationID ids.UUID
	Role         string
	InvitedBy    ids.UUID
}

/*
InvitationAccepted is dispatched in the transaction that accepted the
invitation. MemberID is the membership it created or reactivated, and
UserID the user who accepted.
*/
type InvitationAccepted struct {
	WorkspaceID  ids.UUID
	InvitationID ids.UUID
	MemberID     ids.UUID
	UserID       ids.UUID
}

/*
InvitationRevoked is dispatched in the transaction that withdrew the
invitation. RevokedBy is the member who withdrew it.
*/
type InvitationRevoked struct {
	WorkspaceID  ids.UUID
	InvitationID ids.UUID
	RevokedBy    ids.UUID
}
