/*
|--------------------------------------------------------------------------
| Merge And Bulk Actions
|--------------------------------------------------------------------------
|
| The actions that change many conversations at once and that merge
| conversations or contacts. The bulk change builds its own update and
| trash actions, so it never depends on the order the inbox's actions
| are built in.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/merges"
)

/*
bootMerges builds the bulk change and merge actions.
*/
func bootMerges(a *app.App) {
	acts := a.Actions.Inbox

	acts.BulkChange = &conversations.BulkChangeConversations{
		Apply: &conversations.ApplyConversationChanges{DB: a.DB, Events: a.Events, Clock: a.Clock},
		Trash: &conversations.TrashConversation{DB: a.DB, Events: a.Events, Clock: a.Clock},
	}
	acts.MergeConversations = &merges.MergeConversations{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.MergeContacts = &merges.MergeContacts{DB: a.DB, Events: a.Events, Clock: a.Clock}
}
