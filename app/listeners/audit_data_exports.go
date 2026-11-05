/*
|--------------------------------------------------------------------------
| Audit Log: Data Exports
|--------------------------------------------------------------------------
|
| An export takes conversations or customers out of the workspace, so
| the audit log keeps who asked for which, when the file was ready and
| when it was deleted.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
listenDataExports registers the entries of data exports.
*/
func (l *WriteAuditLog) listenDataExports(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.dataExportRequested)
	events.Listen(d, events.InTx, l.dataExportReady)
	events.Listen(d, events.InTx, l.dataExportDeleted)
}

/*
dataExportSubject names the export.
*/
func dataExportSubject(id ids.UUID) audit.Subject {
	return audit.Subject{Type: "data_export", ID: id, Label: models.PublicID(models.PrefixDataExport, id)}
}

/*
dataExportRequested records a request with what it covers.
*/
func (l *WriteAuditLog) dataExportRequested(ctx context.Context, e appevents.DataExportRequested) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventDataExportRequested, ActorID: e.Actor.ID,
		Subject: dataExportSubject(e.ExportID),
		Changes: map[string]any{"kind": e.Kind, "format": e.Format, "matched": e.Matched},
	})
}

/*
dataExportReady records the file built, with its rows.
*/
func (l *WriteAuditLog) dataExportReady(ctx context.Context, e appevents.DataExportReady) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventDataExportReady, ActorID: ids.Nil(),
		Subject: dataExportSubject(e.ExportID), Changes: map[string]any{"rows": e.Rows},
	})
}

/*
dataExportDeleted records an export deleted.
*/
func (l *WriteAuditLog) dataExportDeleted(ctx context.Context, e appevents.DataExportDeleted) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventDataExportDeleted, ActorID: e.Actor.ID,
		Subject: dataExportSubject(e.ExportID),
	})
}
