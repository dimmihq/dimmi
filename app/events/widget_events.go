/*
|--------------------------------------------------------------------------
| Widget Events
|--------------------------------------------------------------------------
|
| A messenger widget was added, changed or removed. Each is dispatched in
| the transaction that did it. Open settings pages hear about them, and a
| change to the settings drops every replica's cached copy of the widget.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
WidgetCreated says a widget was added, by the member or, with ids.Nil(), by
a new workspace's setup.
*/
type WidgetCreated struct {
	WorkspaceID ids.UUID
	WidgetID    ids.UUID
	ActorID     ids.UUID
}

/*
WidgetSettingsUpdated says a widget's settings changed. Changes names the
fields that changed and Sensitive those that decide who may use it, which
the audit log records with their values.
*/
type WidgetSettingsUpdated struct {
	WorkspaceID ids.UUID
	WidgetID    ids.UUID
	ActorID     ids.UUID
	Name        string
	Changes     []string
	Sensitive   map[string]any
}

/*
WidgetDeleted says a widget was removed.
*/
type WidgetDeleted struct {
	WorkspaceID ids.UUID
	WidgetID    ids.UUID
	ActorID     ids.UUID
	Name        string
}
