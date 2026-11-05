/*
|--------------------------------------------------------------------------
| Delete Draft
|--------------------------------------------------------------------------
|
| A member discards their draft, which the composer does once it holds
| no text, files or recipients. Their other screens clear it too.
|
*/

package drafts

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteInput is who discards which draft.
*/
type DeleteInput struct {
	Membership authz.Membership
	ID         ids.UUID
	MutationID string
}

/*
DeleteDraft discards one of the caller's drafts.

Rules:
  - The caller needs conversations:write.
  - Only the caller's own draft, of a conversation they can see.

Events: [appevents.DraftUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DeleteDraft struct {
	Saver
}

/*
Handle discards the draft.
*/
func (a *DeleteDraft) Handle(ctx context.Context, in DeleteInput) error {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		m := in.Membership

		d, err := own(ctx, q, m, in.ID)
		if err != nil {
			return err
		}

		if _, err := q.DeleteDraft(ctx, models.DeleteDraftParams{
			WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ID: d.ID,
		}); err != nil {
			return fmt.Errorf("delete draft: %w", err)
		}

		return a.tell(ctx, tx, m, appevents.DraftUpdated{
			Base:           appevents.Base{ClientMutationID: in.MutationID},
			ConversationID: d.ConversationID, Mode: d.Mode,
		})
	})
}

/*
Merge names a conversation merged away, the one it merged into, and
when.
*/
type Merge struct {
	Workspace ids.UUID
	Source    ids.UUID
	Target    ids.UUID
	Now       time.Time
}

/*
MoveOnMerge hands the drafts of a conversation merged away to the
conversation it merged into. A member who already drafts the same mode
on the target keeps that one; their draft on the source stays hidden
until the purge removes it.
*/
func MoveOnMerge(ctx context.Context, q *models.Queries, merge Merge) error {
	_, err := q.MoveConversationDrafts(ctx, models.MoveConversationDraftsParams{
		WorkspaceID: merge.Workspace, SourceID: merge.Source, TargetID: merge.Target, Now: merge.Now,
	})
	if err != nil {
		return fmt.Errorf("move drafts: %w", err)
	}

	return nil
}
