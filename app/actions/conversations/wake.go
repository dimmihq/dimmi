/*
|--------------------------------------------------------------------------
| Waking a Snooze
|--------------------------------------------------------------------------
|
| A sleeping conversation wakes when something happens that someone
| should look at. A customer's message always wakes it. What teammates
| do wakes it as the workspace chose: a reply that keeps the status, by
| default; a note by someone other than its holder, or handing it to
| someone else, only when turned on. A reply that snoozes it again keeps
| it asleep.
|
*/

package conversations

import (
	"context"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The reasons teammates' moves wake a conversation.
*/
const (
	ReasonMemberReply = "member_reply"
	ReasonOthersNote  = "others_note"
	ReasonAssigned    = "assigned"
)

/*
WakeIf wakes the snoozed conversation for the reason when the workspace
wakes on it, and returns the conversation as it now is.
*/
func (w *Writer) WakeIf(
	ctx context.Context, ws models.Workspace, c models.Conversation, reason string,
) (models.Conversation, error) {
	if c.Status != string(enums.ConversationStatusSnoozed) || !ws.WakesSnoozeOn(reason) {
		return c, nil
	}

	woken, _, err := w.ChangeStatus(ctx, c, StatusChange{Status: string(enums.ConversationStatusOpen), Reason: reason})

	return woken, err
}
