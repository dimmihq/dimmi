/*
|--------------------------------------------------------------------------
| Sent Drafts
|--------------------------------------------------------------------------
|
| Sending what a member drafted ends the draft in the same transaction:
| a reply ends their reply draft, a note their note draft and a new
| conversation they start their new-conversation draft. Their other
| screens hear the draft went away and clear their composer.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The modes a draft is written in.
*/
const (
	DraftReply = "reply"
	DraftNote  = "note"
	DraftNew   = "new"
)

/*
SentDraft names the draft a send ends: its workspace, its conversation,
nil for a new conversation's draft, and its mode.
*/
type SentDraft struct {
	Workspace    ids.UUID
	Conversation *ids.UUID
	Mode         string
}

/*
ConsumeDraft removes the writer's member's draft the send ends. A writer
acting for no member has no drafts.
*/
func ConsumeDraft(ctx context.Context, w *Writer, sent SentDraft) error {
	if w.Actor.Type != string(enums.AuthorTypeMember) {
		return nil
	}

	gone, err := w.Q.DeleteDraftFor(ctx, models.DeleteDraftForParams{
		WorkspaceID: sent.Workspace, MemberID: w.Actor.ID, ConversationID: sent.Conversation, Mode: sent.Mode,
	})
	if err != nil {
		return fmt.Errorf("remove sent draft: %w", err)
	}

	if len(gone) == 0 {
		return nil
	}

	return w.Dispatch(ctx, appevents.DraftUpdated{
		Base: w.Base(sent.Workspace), MemberID: w.Actor.ID, ConversationID: sent.Conversation, Mode: sent.Mode,
	})
}
