/*
|--------------------------------------------------------------------------
| Assign On Reply
|--------------------------------------------------------------------------
|
| Replying to a conversation nobody holds usually takes it, so the
| customer knows who is on it. Each inbox decides whether that happens,
| and each member may override their inboxes either way. Someone who
| always takes what they answer also takes an unassigned conversation
| they reopen or wake up by hand.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AssignOnReply reports whether the member's reply in the inbox takes a
conversation nobody holds.
*/
func AssignOnReply(member models.WorkspaceMember, inbox models.Inbox) bool {
	switch enums.AssignOnReply(member.AssignOnReply) {
	case enums.AssignOnReplyAlways:
		return true
	case enums.AssignOnReplyNever:
		return false
	case enums.AssignOnReplyInherit:
	}

	return inbox.ReadSettings().AssignOnReply
}

/*
TakeOnReply gives the conversation to the writing member when nobody
holds it and their rule says a reply takes it. A caller with no member
behind it takes nothing.
*/
func (w *Writer) TakeOnReply(
	ctx context.Context, c models.Conversation, member models.WorkspaceMember,
) (models.Conversation, error) {
	if c.AssigneeMemberID != nil || member.ID == ids.Nil() {
		return c, nil
	}

	inbox, err := w.Q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: c.WorkspaceID, ID: c.InboxID})
	if err != nil {
		return c, fmt.Errorf("find inbox: %w", err)
	}

	if !AssignOnReply(member, inbox) {
		return c, nil
	}

	c, _, err = w.Assign(ctx, c, Assignment{Member: &member.ID, Team: c.AssigneeTeamID})

	return c, err
}

/*
takeOnWake gives a conversation the member reopened or woke by hand to
them, when nobody holds it and they always take what they work on.
*/
func (w *Writer) takeOnWake(
	ctx context.Context, from string, c models.Conversation, member models.WorkspaceMember,
) (models.Conversation, error) {
	woken := from != c.Status && c.Status == string(enums.ConversationStatusOpen)
	if !woken || c.AssigneeMemberID != nil || enums.AssignOnReply(member.AssignOnReply) != enums.AssignOnReplyAlways {
		return c, nil
	}

	c, _, err := w.Assign(ctx, c, Assignment{Member: &member.ID, Team: c.AssigneeTeamID})

	return c, err
}
