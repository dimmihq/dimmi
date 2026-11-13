/*
|--------------------------------------------------------------------------
| Widget Actions
|--------------------------------------------------------------------------
|
| The messenger's actions, built once at boot, with the services they share:
| the directory of widgets and the built-in words.
|
*/

package app

import (
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/support/widgetinstall"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
)

/*
WidgetActions holds the messenger's actions.
*/
type WidgetActions struct {
	Directory    *widget.Directory
	Strings      *widgetstrings.Strings
	ShowSettings *widget.ShowWidgetSettings
	Create       *widget.CreateWidget
	Update       *widget.UpdateWidgetSettings
	Delete       *widget.DeleteWidget
	Branding     *widget.ResolveBranding
	ShowConfig   *widget.ShowWidgetConfig
	Surfaces     *widget.Surfaces
	Preview      *widget.PreviewWidgetConfig
	Availability *widget.ComputeTeamAvailability
	Install      *widgetinstall.Tracker

	Authenticate   *widget.AuthenticateWidgetSession
	StartSession   *widget.StartWidgetSession
	ShowSession    *widget.ShowWidgetSession
	RevokeSession  *widget.RevokeWidgetSession
	UpdateContact  *widget.UpdateWidgetContact
	EmailProofs    *widget.EmailProofs
	Conversations  *widget.ListWidgetConversations
	PostMessage    *widget.PostWidgetMessage
	OpenChat       *widget.OpenWidgetConversation
	MarkRead       *widget.MarkWidgetConversationRead
	ConfirmHelp    *widget.ConfirmAiHelp
	RequestHandoff *widget.RequestAiHandoff
	Typing         *widget.BroadcastVisitorTyping
	Ping           *widget.RecordWidgetPing
	Realtime       *widget.Realtime
	Updates        *widget.ListWidgetUpdates
	Contacts       *widget.ContactSessions
	Uploads        *widget.Uploads
}
