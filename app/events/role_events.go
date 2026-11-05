/*
|--------------------------------------------------------------------------
| Role Events
|--------------------------------------------------------------------------
|
| A workspace's custom roles changed, or a member's custom role did.
| Admins' role pickers refresh on these, and holders of an edited role
| work under its new list from their very next request.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
RoleCreated is dispatched in the transaction that created the custom
role. CreatedBy is the member who created it.
*/
type RoleCreated struct {
	WorkspaceID ids.UUID
	RoleID      ids.UUID
	Name        string
	BaseRole    string
	Permissions []string
	CreatedBy   ids.UUID
}

/*
RoleUpdated is dispatched in the transaction that changed the custom
role. Changes holds, per field, what it was and what it became, and
UpdatedBy is the member who changed it.
*/
type RoleUpdated struct {
	WorkspaceID ids.UUID
	RoleID      ids.UUID
	Name        string
	Changes     map[string]any
	UpdatedBy   ids.UUID
}

/*
RoleDeleted is dispatched in the transaction that deleted the custom
role. DeletedBy is the member who deleted it.
*/
type RoleDeleted struct {
	WorkspaceID ids.UUID
	RoleID      ids.UUID
	Name        string
	DeletedBy   ids.UUID
}

/*
MemberCustomRoleChanged is dispatched in the transaction that gave a
member a custom role or took it away. From and To are the custom roles
before and after, nil for none, and ChangedBy the member who changed it.
*/
type MemberCustomRoleChanged struct {
	WorkspaceID ids.UUID
	MemberID    ids.UUID
	UserID      ids.UUID
	From        *ids.UUID
	To          *ids.UUID
	ChangedBy   ids.UUID
}
