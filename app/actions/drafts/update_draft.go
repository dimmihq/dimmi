/*
|--------------------------------------------------------------------------
| Update Draft
|--------------------------------------------------------------------------
|
| A member replaces what one of their drafts holds by its id. As with
| every save, the newest change wins and a stale one writes nothing.
|
*/

package drafts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateInput is who changes which draft, and the content it now holds.
*/
type UpdateInput struct {
	Membership authz.Membership
	ID         ids.UUID
	Content    Content
	MutationID string
}

/*
UpdateDraft replaces the content of one of the caller's drafts.

Rules:
  - The caller needs conversations:write.
  - Only the caller's own draft, of a conversation they can see.
  - The body is 64 KB at most; up to ten files of the workspace.
  - A save older than the stored draft writes nothing.

Events: [appevents.DraftUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDraftTooLarge], [exceptions.ErrDraftAttachments], or a
wrapped database error.
*/
type UpdateDraft struct {
	Saver
}

/*
Handle updates the draft.
*/
func (a *UpdateDraft) Handle(ctx context.Context, in UpdateInput) (Saved, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Saved{}, err
	}

	var out Saved

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		d, err := own(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		if err := check(ctx, q, in.Membership.Workspace.ID, in.Content); err != nil {
			return err
		}

		out, err = a.write(ctx, tx, edit{m: in.Membership, d: d, c: in.Content, mutation: in.MutationID})

		return err
	})

	return out, err
}
