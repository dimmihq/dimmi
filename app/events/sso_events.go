/*
|--------------------------------------------------------------------------
| Single Sign-On Events
|--------------------------------------------------------------------------
|
| A company's connections to its identity provider and its proven
| domains changed. Each lands in the audit log of the workspace it was
| changed from, since who may sign in, and how, is exactly what a
| security review asks about.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
SSOConnectionChanged is dispatched in the transaction that changed a
connection. Change is created, updated, activated, enforced, unenforced
or deleted; Changes holds, per field, what it was and what it became.
ChangedBy is the member who did it, ids.Nil() for the system.
*/
type SSOConnectionChanged struct {
	WorkspaceID  ids.UUID
	ConnectionID ids.UUID
	Name         string
	Change       string
	Changes      map[string]any
	ChangedBy    ids.UUID
}

/*
OrganizationDomainChanged is dispatched in the transaction that changed
a domain. Change is added, verified, failed, routed or removed.
ChangedBy is the member who did it, ids.Nil() for the system.
*/
type OrganizationDomainChanged struct {
	WorkspaceID ids.UUID
	DomainID    ids.UUID
	Domain      string
	Change      string
	Changes     map[string]any
	ChangedBy   ids.UUID
}
