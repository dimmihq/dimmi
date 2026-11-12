/*
|--------------------------------------------------------------------------
| Merge And Bulk Routes
|--------------------------------------------------------------------------
|
| Changing many conversations at once and merging conversations or
| contacts. They sit with the inbox's routes, behind the same guards.
| Bulk changes and conversation merges take conversations:write, a
| contact merge contacts:write; the actions check the rest.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
mergeRoutes declares the bulk change and merge routes.
*/
func mergeRoutes(r *routing.Router, a *app.App, c inboxControllers) {
	acts := a.Actions.Inbox
	m := &v1.MergeController{
		Bulk: acts.BulkChange, Conversations: c.conversations, Merge: acts.MergeConversations,
		MergeContacts: acts.MergeContacts, ShowContact: acts.ShowContact, IDs: a.IDs,
	}

	routing.Post(r, "/conversation-bulk-changes", "conversation-bulk-changes.store", m.StoreBulk).
		Ability("conversations:write").Status(fhttp.StatusOK)
	routing.Post(r, "/conversations/{conversation}/merges", "conversations.merges.store",
		m.StoreConversationMerge).Ability("conversations:write").Status(fhttp.StatusOK)
	routing.Post(r, "/contacts/{contact}/merges", "contacts.merges.store", m.StoreContactMerge).
		Ability("contacts:write").Status(fhttp.StatusOK)
}
