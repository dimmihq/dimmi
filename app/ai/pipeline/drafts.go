/*
|--------------------------------------------------------------------------
| Drafts
|--------------------------------------------------------------------------
|
| Where the team reviews the agent before the customer reads it, an
| answer becomes a draft: kept on the run, never posted, never counted
| as an answer, and handed to the channel to show the team. A teammate
| who sends it makes it their own reply. A run held back so a teammate
| could answer first, and answered meanwhile, says so when it skips.
|
*/

package pipeline

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
drafting reports whether the conversation's channel wants a draft.
*/
func (r *Runner) drafting(ctx context.Context, e *env) bool {
	if r.Drafts == nil {
		return false
	}
	return r.Drafts.DraftsFor(ctx, e.Conversation.WorkspaceID, e.Conversation.ID)
}

/*
writeDraft keeps the answer on the run and hands it to the channel.
*/
func (r *Runner) writeDraft(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	e := st.env
	d.outcome = outcomeDrafted
	err := models.New(tx).SetRunDraft(ctx, models.SetRunDraftParams{
		DraftBody: new(r.reply(e, d)), Now: r.Clock.Now(), WorkspaceID: e.Conversation.WorkspaceID,
		ID: st.rec.Run.ID,
	})
	if err != nil {
		return fmt.Errorf("keep draft: %w", err)
	}
	return r.Drafts.Drafted(ctx, tx, contracts.AgentDraft{
		WorkspaceID: e.Conversation.WorkspaceID, ConversationID: e.Conversation.ID, RunID: st.rec.Run.ID,
	})
}

/*
inDelay names a teammate's answer during the channel's delay as such.
*/
func inDelay(d *decision, args Args) *decision {
	if d != nil && args.Delayed && d.skipReason == "human_replied" {
		d.skipReason = "human_replied_in_delay"
	}
	return d
}
