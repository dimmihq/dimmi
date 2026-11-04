/*
|--------------------------------------------------------------------------
| Single Sign-On Audit Entries
|--------------------------------------------------------------------------
|
| Who connected an identity provider, enforced it, proved a domain or
| switched it all off is what a security review asks first, so every such
| change lands in the audit log of the workspace it came from. Operators
| and the daily domain check act as the system. Secrets never appear,
| only that they changed.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
listenSSO registers the entries of connections and domains.
*/
func (l *WriteAuditLog) listenSSO(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.ssoConnectionChanged)
	events.Listen(d, events.InTx, l.organizationDomainChanged)
}

/*
ssoConnectionChanged records a change to a connection.
*/
func (l *WriteAuditLog) ssoConnectionChanged(ctx context.Context, e appevents.SSOConnectionChanged) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventSSOConnection + e.Change, ActorID: e.ChangedBy,
		Subject: audit.Subject{Type: subjectConnection, ID: e.ConnectionID, Label: e.Name}, Changes: e.Changes,
	})
}

/*
organizationDomainChanged records a change to a domain.
*/
func (l *WriteAuditLog) organizationDomainChanged(ctx context.Context, e appevents.OrganizationDomainChanged) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventOrganizationDomain + e.Change, ActorID: e.ChangedBy,
		Subject: audit.Subject{Type: subjectDomain, ID: e.DomainID, Label: e.Domain}, Changes: e.Changes,
	})
}
