/*
|--------------------------------------------------------------------------
| Audit Widgets
|--------------------------------------------------------------------------
|
| Who added or removed a messenger, and who changed who may use one. Changes
| to its look are not worth an entry; changes to its allowed sites are, with
| their values. Changes to who may open an inbox are recorded here too, as
| the other half of who sees what a channel brings in.
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
The kinds of subject the entries written here are about.
*/
const (
	subjectWidget = "widget"
	subjectInbox  = "inbox"
)

/*
listenWidgets registers the entries of widgets.
*/
func (l *WriteAuditLog) listenWidgets(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.widgetSettingsUpdated)
	events.Listen(d, events.InTx, l.widgetDeleted)
	events.Listen(d, events.InTx, l.inboxAccessChanged)
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

/*
inboxAccessChanged records who may now open an inbox.
*/
func (l *WriteAuditLog) inboxAccessChanged(ctx context.Context, e appevents.InboxAccessChanged) error {
	return l.record(ctx, audit.Entry{
		WorkspaceID: e.WorkspaceID, Event: audit.EventInboxAccessChanged, ActorID: e.ActorID,
		Subject: audit.Subject{Type: subjectInbox, ID: e.InboxID, Label: e.Name},
		Changes: map[string]any{"access": map[string]any{
			"to": map[string]any{
				"mode": e.Access, "team_ids": publicOf(models.PrefixTeam, e.Teams),
				"member_ids": publicOf(models.PrefixMember, e.Members),
			},
		}},
	})
}

/*
publicOf returns the public ids of the rows.
*/
func publicOf(prefix models.Prefix, rows []ids.UUID) []string {
	out := make([]string, len(rows))
	for i, id := range rows {
		out[i] = models.PublicID(prefix, id)
	}

	return out
}
