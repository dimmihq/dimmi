/*
|--------------------------------------------------------------------------
| Draft Events
|--------------------------------------------------------------------------
|
| A draft is its writer's alone, so a change to one reaches only their
| own open screens, never the workspace and never a webhook. Another tab
| reads the draft again unless its composer holds unsaved typing.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
DraftUpdated is dispatched when a member saves, discards or sends a
draft. ConversationID is nil for a new conversation's draft; DraftID is
nil once the draft is gone.
*/
type DraftUpdated struct {
	Base

	MemberID       ids.UUID
	ConversationID *ids.UUID
	Mode           string
	DraftID        *ids.UUID
}

/*
EventName names the event.
*/
func (DraftUpdated) EventName() string { return "draft.updated" }
