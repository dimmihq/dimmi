/*
|--------------------------------------------------------------------------
| Drafts
|--------------------------------------------------------------------------
|
| A draft is the answer a member is still writing, kept so it follows
| them to another device. Each member has at most one draft per
| conversation and mode, and one new conversation draft. A draft is
| never anyone else's business: another member's draft is not found,
| whatever their role, and so is a draft whose conversation the member
| can no longer see. It stays stored, and shows again if they can.
|
*/

package drafts

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxListed is the most drafts one list returns.
*/
const maxListed = 100

/*
ListInput is whose drafts to list: all of them, or those of one
conversation or one mode when set.
*/
type ListInput struct {
	Membership   authz.Membership
	Conversation *ids.UUID
	Mode         *string
}

/*
ListDrafts returns the caller's drafts, newest first.

Rules:
  - The caller needs conversations:read.
  - A conversation filter the caller cannot see is not found.
  - Drafts of conversations in the trash, or that the caller may no
    longer see, are left out.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListDrafts struct {
	DB *database.DB
}

/*
Handle returns the drafts.
*/
func (a *ListDrafts) Handle(ctx context.Context, in ListInput) ([]models.ConversationDraft, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return nil, err
	}

	var out []models.ConversationDraft

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ws := in.Membership.Workspace.ID

		if in.Conversation != nil {
			ref := conversations.Ref{ID: *in.Conversation}
			if _, err := conversations.Find(ctx, q, in.Membership, ref); err != nil {
				return err
			}
		}

		access := conversations.VisibleScope(in.Membership).Params()

		rows, err := q.ListMemberDrafts(ctx, models.ListMemberDraftsParams{
			WorkspaceID: ws, MemberID: in.Membership.Member.ID,
			ConversationID: in.Conversation, Mode: in.Mode, RowLimit: maxListed,
			AccessMode: access.Mode, AccessViewer: access.Viewer, AccessTeams: access.Teams,
			AccessUnassigned: access.Unassigned, AccessHiddenInboxes: access.HiddenInboxes,
		})
		if err != nil {
			return fmt.Errorf("list drafts: %w", err)
		}

		out = rows

		return nil
	})

	return out, err
}

/*
own returns the caller's draft of the id, refusing a draft that is not
theirs or whose conversation they cannot see.
*/
func own(ctx context.Context, q *models.Queries, m authz.Membership, id ids.UUID) (models.ConversationDraft, error) {
	d, err := q.FindMemberDraft(ctx, models.FindMemberDraftParams{
		WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return d, exceptions.ErrNotFound
	}

	if err != nil {
		return d, fmt.Errorf("find draft: %w", err)
	}

	return d, reachable(ctx, q, m, d.ConversationID)
}

/*
reachable refuses a conversation the caller cannot see; a new
conversation's draft has none.
*/
func reachable(ctx context.Context, q *models.Queries, m authz.Membership, conversation *ids.UUID) error {
	if conversation == nil {
		return nil
	}

	_, err := conversations.Find(ctx, q, m, conversations.Ref{ID: *conversation})

	return err
}
