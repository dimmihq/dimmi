/*
|--------------------------------------------------------------------------
| Inbox Routes
|--------------------------------------------------------------------------
|
| The shared inbox's part of the API: inboxes, contacts, conversations
| and their timelines, counts, read state and search. Every route works
| inside the workspace the request names, behind the same guards as the
| rest of the workspace routes. Reading takes conversations:read or
| contacts:read, writing their write abilities; managing inboxes is a
| settings change.
|
*/

package routes

import (
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
inboxControllers are the inbox's controllers, built once with the
actions they call.
*/
type inboxControllers struct {
	inboxes       *v1.InboxController
	contacts      *v1.ContactController
	conversations *v1.ConversationController
	messages      *v1.ConversationMessageController
	views         *v1.InboxViewController
}

/*
newInboxControllers builds the inbox's controllers.
*/
func newInboxControllers(a *app.App) inboxControllers {
	acts, cursors := a.Actions.Inbox, pagination.NewCursors(a.DerivedKey(v1.CursorKeyPurpose))

	return inboxControllers{
		inboxes: &v1.InboxController{
			List: acts.ListInboxes, Show: acts.ShowInbox, Create: acts.CreateInbox, Change: acts.UpdateInbox,
			IDs: a.IDs,
		},
		contacts: &v1.ContactController{
			List: acts.ListContacts, Show: acts.ShowContact, Create: acts.CreateContact, Upsert: acts.UpsertContact,
			Change: acts.UpdateContact, Conversations: acts.ListRecordConversations,
			IDs: a.IDs, Cursors: cursors,
		},
		conversations: &v1.ConversationController{
			List: acts.ListConversations, Show: acts.ShowConversation, Create: acts.CreateConversation,
			Apply: acts.ApplyChanges, Neighbors: acts.ShowNeighbors, Viewers: a.Viewers, IDs: a.IDs,
			Cursors: cursors,
		},
		messages: &v1.ConversationMessageController{
			List: acts.ListMessages, Reply: acts.Reply, Note: acts.AddNote, Relay: acts.AddContactMessage, IDs: a.IDs,
			Cursors: cursors,
		},
		views: &v1.InboxViewController{
			Counts: acts.CountConversations, ReadShow: acts.ShowReadState, ReadSave: acts.UpdateReadState,
			Search: acts.Search, IDs: a.IDs, Clock: a.Clock,
		},
	}
}

/*
inboxRoutes declares the inbox's routes.
*/
func inboxRoutes(r *routing.Router, a *app.App) {
	r.Middleware("csrf", "session", "auth", "workspace", "api.throttle")

	c := newInboxControllers(a)

	routing.Get(r, "/inboxes", "inboxes.index", c.inboxes.Index).Ability("conversations:read")
	routing.Post(r, "/inboxes", "inboxes.store", c.inboxes.Store).Ability("settings:write")
	routing.Get(r, "/inboxes/{inbox}", "inboxes.show", c.inboxes.Get).Ability("conversations:read")
	routing.Patch(r, "/inboxes/{inbox}", "inboxes.update", c.inboxes.Update).Ability("settings:write")

	routing.Get(r, "/contacts", "contacts.index", c.contacts.Index).Ability("contacts:read")
	routing.Post(r, "/contacts", "contacts.store", c.contacts.Store).Ability("contacts:write")
	routing.Get(r, "/contacts/{contact}", "contacts.show", c.contacts.Get).Ability("contacts:read")
	routing.Patch(r, "/contacts/{contact}", "contacts.update", c.contacts.Update).Ability("contacts:write")

	conversationRoutes(r, c)

	routing.Get(r, "/conversation-counts", "conversation-counts.index", c.views.CountIndex).
		Ability("conversations:read")
	routing.Get(r, "/search", "search.index", c.views.SearchIndex).Ability("conversations:read")

	inboxViewRoutes(r, a)
}

/*
conversationRoutes declares the routes on conversations and their
timelines.
*/
func conversationRoutes(r *routing.Router, c inboxControllers) {
	conv := "/conversations/{conversation}"

	routing.Get(r, "/conversations", "conversations.index", c.conversations.Index).Ability("conversations:read")
	routing.Post(r, "/conversations", "conversations.store", c.conversations.Store).Ability("conversations:write")
	routing.Get(r, conv, "conversations.show", c.conversations.Get).Ability("conversations:read")
	routing.Patch(r, conv, "conversations.update", c.conversations.Update).Ability("conversations:write")
	routing.Get(r, conv+"/neighbors", "conversations.neighbors.index", c.conversations.ListNeighbors).
		Ability("conversations:read")
	routing.Get(r, conv+"/read-state", "conversations.read-state.show", c.views.ReadState).
		Ability("conversations:read")
	routing.Patch(r, conv+"/read-state", "conversations.read-state.update", c.views.UpdateReadState).
		Ability("conversations:read")
	routing.Get(r, conv+"/messages", "conversations.messages.index", c.messages.Index).
		Ability("conversations:read")
	routing.Post(r, conv+"/messages", "conversations.messages.store", c.messages.Store).
		Ability("conversations:write")
	routing.Post(r, conv+"/replies", "conversations.replies.store", c.messages.StoreReply).
		Ability("conversations:write")
	routing.Post(r, conv+"/notes", "conversations.notes.store", c.messages.StoreNote).
		Ability("conversations:write")
}
