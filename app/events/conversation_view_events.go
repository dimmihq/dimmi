/*
|--------------------------------------------------------------------------
| Conversation View Events
|--------------------------------------------------------------------------
|
| Saved views change for whoever sees them: a shared view's change goes
| to the whole workspace, a personal view's only to its owner, so every
| open sidebar shows the same views.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
ConversationViewChanged is dispatched when a saved view is created,
updated or deleted, as Action says. OwnerMemberID is set for a personal
view.
*/
type ConversationViewChanged struct {
	Base

	ViewID        ids.UUID
	Action        string
	OwnerMemberID *ids.UUID
}

/*
EventName names the event after what happened to the view.
*/
func (e ConversationViewChanged) EventName() string { return "conversation_view." + e.Action }
