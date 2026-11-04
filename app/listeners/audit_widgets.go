/*
|--------------------------------------------------------------------------
| Audit Widgets
|--------------------------------------------------------------------------
|
| Who added or removed a messenger, and who changed who may use one. Changes
| to its look are not worth an entry; changes to its allowed sites are, with
| their values.
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
subjectWidget is the kind of subject the entries written here are about.
*/
const subjectWidget = "widget"

/*
listenWidgets registers the entries of widgets.
*/
func (l *WriteAuditLog) listenWidgets(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.widgetSettingsUpdated)
	events.Listen(d, events.InTx, l.widgetDeleted)
}

/*
widgetSettingsUpdated records a change to who may use a widget.
*/
func (l *WriteAuditLog) widgetSettingsUpdated(ctx context.Context, e appevents.WidgetSettingsUpdated) error {
	if len(e.Sensitive) == 0 {
		return nil
	}

	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventWidgetSettingsUpdated, ActorID: e.ActorID,
		Subject: audit.Subject{Type: subjectWidget, ID: e.WidgetID, Label: e.Name}, Changes: e.Sensitive,
	})
}

/*
widgetDeleted records a removed widget.
*/
func (l *WriteAuditLog) widgetDeleted(ctx context.Context, e appevents.WidgetDeleted) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventWidgetDeleted, ActorID: e.ActorID,
		Subject: audit.Subject{Type: subjectWidget, ID: e.WidgetID, Label: e.Name},
	})
}
