/*
|--------------------------------------------------------------------------
| View Broadcasts
|--------------------------------------------------------------------------
|
| A change to a saved view reaches whoever sees the view: the whole
| workspace for a shared view, only its owner for a personal one. The
| broadcast names the view, and screens read the views again. A change
| to the snooze choices reaches the whole workspace.
|
*/

package broadcasting

import (
	"context"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
viewChanged tells the screens that see the view it changed.
*/
func (b *Broadcaster) viewChanged(ctx context.Context, e appevents.ConversationViewChanged) error {
	channel := WorkspaceChannel(e.WorkspaceID)
	if e.OwnerMemberID != nil {
		channel = MemberChannel(*e.OwnerMemberID)
	}

	return b.publish(ctx, []string{channel}, message(e, map[string]string{
		"id": models.PublicID(models.PrefixConversationView, e.ViewID), "object": "conversation_view",
	}))
}

/*
snoozeListChanged tells the workspace's screens the snooze choices
changed.
*/
func (b *Broadcaster) snoozeListChanged(ctx context.Context, e appevents.SnoozeListUpdated) error {
	data := map[string]any{"team_id": nil}
	if e.TeamID != nil {
		data["team_id"] = models.PublicID(models.PrefixTeam, *e.TeamID)
	}

	return b.publish(ctx, []string{WorkspaceChannel(e.WorkspaceID)}, message(e, data))
}
