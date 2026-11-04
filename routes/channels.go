/*
|--------------------------------------------------------------------------
| Channel Routes
|--------------------------------------------------------------------------
|
| Who may listen on which realtime channel. Each pattern names a
| namespace and the public id it follows; its authorizer runs on every
| subscription, with the caller the connection authenticated as. A
| channel no pattern matches is refused.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/app"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
)

/*
Channels declares the realtime channel authorizers.
*/
func Channels(a *app.App) {
	channels := a.Broadcast.Channels()
	authorize := &appbroadcasting.Authorizers{
		DB: a.DB, IDs: a.IDs, Resolve: a.Actions.ResolveWorkspace,
	}

	channels.MustAuthorize(appbroadcasting.WorkspacePattern, appbroadcasting.WorkspaceOptions(), authorize.Workspace)
	channels.MustAuthorize(appbroadcasting.WorkspaceConversationsPattern,
		appbroadcasting.WorkspaceConversationsOptions(), authorize.WorkspaceConversations)
	channels.MustAuthorize(appbroadcasting.ConversationPattern, appbroadcasting.ConversationOptions(),
		authorize.Conversation)
	channels.MustAuthorize(appbroadcasting.MemberPattern, appbroadcasting.MemberOptions(), authorize.Member)
	channels.MustAuthorize(appbroadcasting.WidgetContactPattern, appbroadcasting.WidgetContactOptions(),
		authorize.WidgetContact)
}
