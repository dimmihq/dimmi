/*
|--------------------------------------------------------------------------
| Company Events
|--------------------------------------------------------------------------
|
| A company joined the workspace, its details changed, or it was deleted.
| Each is dispatched in the transaction that made the change; a change
| that changed nothing dispatches nothing.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
CompanyCreated says a company was added by the member.
*/
type CompanyCreated struct {
	WorkspaceID ids.UUID
	CompanyID   ids.UUID
	ActorID     ids.UUID
}

/*
CompanyUpdated says a company's details changed. Changes lists the
fields that changed and ChangedAttributeKeys the custom attributes
written or removed.
*/
type CompanyUpdated struct {
	WorkspaceID          ids.UUID
	CompanyID            ids.UUID
	ActorID              ids.UUID
	Changes              []string
	ChangedAttributeKeys []string
}

/*
CompanyDeleted says a company was deleted by the member.
*/
type CompanyDeleted struct {
	WorkspaceID ids.UUID
	CompanyID   ids.UUID
	ActorID     ids.UUID
}
