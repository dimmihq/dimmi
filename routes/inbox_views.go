/*
|--------------------------------------------------------------------------
| Inbox View Routes
|--------------------------------------------------------------------------
|
| How each teammate arranges their inbox: their own preferences for the
| sidebar and every view, and the saved views the team shares or each
| member keeps, the snooze choices, and each member's drafts. A
| preference is the caller's own, never data others read, so reading and
| saving it takes conversations:read; saving views and drafts takes
| conversations:write.
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
inboxViewRoutes declares the routes of inbox preferences.
*/
func inboxViewRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox
	prefs := &v1.InboxPreferenceController{Show: acts.ShowPreferences, Update: acts.UpdatePreferences}

	routing.Get(r, "/inbox-preferences", "inbox-preferences.show", prefs.Get).Ability("conversations:read")
	routing.Patch(r, "/inbox-preferences", "inbox-preferences.update", prefs.Patch).Ability("conversations:read")

	views := &v1.ConversationViewController{
		List: acts.ListViews, Show: acts.ShowView, Create: acts.CreateView, Update: acts.UpdateView,
		Delete: acts.DeleteView, Reorder: acts.ReorderViews, IDs: a.IDs,
	}
	view := "/conversation-views/{view}"

	routing.Get(r, "/conversation-views", "conversation-views.index", views.Index).Ability("conversations:read").
		Query(routing.QueryParam{Name: "q", Description: "Only views whose name holds this text."})
	routing.Post(r, "/conversation-views", "conversation-views.store", views.Store).Ability("conversations:write")
	routing.Patch(r, "/conversation-views", "conversation-views.reorder", views.ReorderShared).
		Ability("conversations:write")
	routing.Get(r, view, "conversation-views.show", views.Get).Ability("conversations:read")
	routing.Patch(r, view, "conversation-views.update", views.Patch).Ability("conversations:write")
	routing.Delete(r, view, "conversation-views.destroy", views.Destroy).Ability("conversations:write").
		Status(fhttp.StatusNoContent)

	snooze := &v1.SnoozeController{
		Lists: acts.ListSnoozeLists, Update: acts.UpdateSnoozeList, Options: acts.ShowSnoozeOptions, IDs: a.IDs,
	}

	routing.Get(r, "/snooze-lists", "snooze-lists.index", snooze.Index).Ability("conversations:read")
	routing.Patch(r, "/snooze-lists/{scope}", "snooze-lists.update", snooze.Patch).Ability("settings:write")
	routing.Get(r, "/snooze-options", "snooze-options.index", snooze.Choices).Ability("conversations:read").
		Query(routing.QueryParam{Name: "conversation", Description: "ID or number of the conversation to snooze."})

	collaborationRoutes(r, a)

	drafts := &v1.DraftController{
		List: acts.ListDrafts, Save: acts.SaveDraft, Update: acts.UpdateDraft, Delete: acts.DeleteDraft, IDs: a.IDs,
	}

	routing.Get(r, "/drafts", "drafts.index", drafts.Index).Ability("conversations:read").Query(
		routing.QueryParam{Name: "filter[conversation]", Description: "Only drafts on this conversation."},
		routing.QueryParam{
			Name: "filter[mode]", Description: "Only drafts of this kind.", Values: []string{"reply", "note", "new"},
		},
	)
	routing.Post(r, "/drafts", "drafts.store", drafts.Store).Ability("conversations:write")
	routing.Patch(r, "/drafts/{draft}", "drafts.update", drafts.Patch).Ability("conversations:write")
	routing.Delete(r, "/drafts/{draft}", "drafts.destroy", drafts.Destroy).Ability("conversations:write").
		Status(fhttp.StatusNoContent)
}

/*
collaborationRoutes declares how teammates work together on a note: who
it may mention, emoji reactions, and the GIF search the composer uses.
*/
func collaborationRoutes(r *routing.Router, a *app.App) {
	acts := a.Actions.Inbox

	mentionables := &v1.MentionableController{List: acts.ListMentionables, IDs: a.IDs}
	routing.Get(r, "/conversations/{conversation}/mentionables", "conversations.mentionables.index",
		mentionables.Index).Ability("conversations:read")

	reactions := &v1.ReactionController{React: acts.ReactToNote, IDs: a.IDs}
	message := "/conversations/{conversation}/messages/{message}/reactions"
	routing.Post(r, message, "conversations.messages.reactions.store", reactions.Store).
		Ability("conversations:write").Status(fhttp.StatusOK)
	routing.Delete(r, message+"/{emoji}", "conversations.messages.reactions.destroy", reactions.Destroy).
		Ability("conversations:write").Status(fhttp.StatusOK)

	gifs := &v1.GIFSearchController{APIKey: a.Config.Inbox.GIF.GiphyAPIKey}
	routing.Get(r, "/gif-search", "gif-search.show", gifs.Show).Ability("conversations:write")
}
