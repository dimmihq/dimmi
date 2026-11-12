/*
|--------------------------------------------------------------------------
| List Saved Replies
|--------------------------------------------------------------------------
|
| A member sees the shared replies and their own, by title, narrowed by
| a search on the title or shortcut.
|
*/

package savedreplies

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListSavedRepliesInput is who asks and what to search for.
*/
type ListSavedRepliesInput struct {
	Membership authz.Membership
	Search     *string
}

/*
ListSavedReplies lists the replies the caller may use.

Rules:
  - The caller needs conversations:read.
  - At most MaxListed come back.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListSavedReplies struct {
	DB *database.DB
}

/*
Handle returns the replies by title.
*/
func (a *ListSavedReplies) Handle(ctx context.Context, in ListSavedRepliesInput) ([]models.SavedReply, error) {
	if !in.Membership.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var search *string
	if in.Search != nil {
		if s := strings.TrimSpace(*in.Search); s != "" {
			search = &s
		}
	}

	var out []models.SavedReply

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListSavedReplies(ctx, models.ListSavedRepliesParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberID: in.Membership.Member.ID, Search: search,
			RowLimit: MaxListed,
		})
		if err != nil {
			return fmt.Errorf("list saved replies: %w", err)
		}

		out = rows

		return nil
	})

	return out, err
}

/*
ShowSavedReply returns one reply the caller may use.

Rules:
  - The caller needs conversations:read; another member's personal
    reply is not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a wrapped
database error.
*/
type ShowSavedReply struct {
	DB *database.DB
}

/*
Handle returns the reply.
*/
func (a *ShowSavedReply) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (models.SavedReply, error) {
	if !m.Can("conversations:read") {
		return models.SavedReply{}, exceptions.ErrForbidden
	}

	var out models.SavedReply

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := find(ctx, models.New(tx), m, id)
		out = row

		return err
	})

	return out, err
}
