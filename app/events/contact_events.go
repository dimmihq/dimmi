/*
|--------------------------------------------------------------------------
| Contact Events
|--------------------------------------------------------------------------
|
| A customer joined the workspace's contacts, or their details changed.
| Each is dispatched in the transaction that made the change; a change
| that changed nothing dispatches nothing.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ContactCreated says a contact was added by the member, ids.Nil() when
no member did.
*/
type ContactCreated struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
	ActorID     ids.UUID
}

/*
ContactUpdated says a contact's details changed. Changes lists the
fields that changed and ChangedAttributeKeys the custom attributes
written or removed. Blocked and Unblocked say whether the change
blocked or unblocked them.
*/
type ContactUpdated struct {
	WorkspaceID          ids.UUID
	ContactID            ids.UUID
	ActorID              ids.UUID
	Changes              []string
	ChangedAttributeKeys []string
	Blocked              bool
	Unblocked            bool
}

/*
WidgetEmailProven says a messenger visitor proved they own the email
address by typing the code mailed to it, in the session named.
*/
type WidgetEmailProven struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
	SessionID   ids.UUID
	Email       string
}
