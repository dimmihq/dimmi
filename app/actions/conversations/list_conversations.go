/*
|--------------------------------------------------------------------------
| List Conversations
|--------------------------------------------------------------------------
|
| The inbox's lists: the caller's own conversations, those nobody holds,
| everyone's, one teammate's, a team's, an inbox's, spam and the trash,
| each narrowed by the filters and put in one of the fixed orders. Pages
| follow each other by the last row's position in the order, so a
| conversation arriving while someone pages never makes a row repeat or
| vanish.
|
| Spam never shows outside the spam view, nor deleted conversations
| outside the trash, which takes conversations:delete. Mentions have
| their own view, which stays empty until mentions are recorded.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
ListConversationsInput is who asks and which page of which list.
DefaultSort says the request chose no order, so a saved view's own
order applies.
*/
type ListConversationsInput struct {
	Membership  authz.Membership
	Filters     Filters
	Sort        querybuilder.Sort
	DefaultSort bool
	After       *querybuilder.Keyset
	Limit       int
}

/*
ConversationPage is a page of conversations, the position of its last
row, whether more follow, and for a saved view the hash of the version
listed.
*/
type ConversationPage struct {
	Conversations []Loaded
	Last          *querybuilder.Keyset
	More          bool
	ViewHash      string
}

/*
ListConversations lists a page of the workspace's conversations.

Rules:
  - The caller needs conversations:read, and conversations:delete for
    the trash.
  - Deleted conversations show only in the trash; spam only in the spam
    view.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound] for a saved
view the caller may not see, [exceptions.ErrInvalidDefinition], or a
wrapped database error.
*/
type ListConversations struct {
	DB    *database.DB
	IDs   *ids.Registry
	Clock clock.Clock
}

/*
Handle returns the page.
*/
func (a *ListConversations) Handle(ctx context.Context, in ListConversationsInput) (ConversationPage, error) {
	if err := canList(in.Membership, in.Filters); err != nil {
		return ConversationPage{}, err
	}

	var page ConversationPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		view, err := a.applyCustomView(ctx, q, &in)
		if err != nil {
			return err
		}

		list, ok := listOf(in.Membership, in.Filters, in.Sort)
		if !ok {
			return nil
		}

		view.apply(&list)
		list.Limit, list.After = in.Limit+1, in.After

		page, err = a.page(ctx, tx, in, list)
		if view != nil {
			page.ViewHash = view.hash
		}

		return err
	})

	return page, err
}

/*
page runs the list and loads its rows.
*/
func (a *ListConversations) page(
	ctx context.Context, tx *database.Tx, in ListConversationsInput, list querybuilder.Conversations,
) (ConversationPage, error) {
	rows, err := selectConversations(ctx, tx, list)
	if err != nil {
		return ConversationPage{}, fmt.Errorf("list conversations: %w", err)
	}

	more := len(rows) > in.Limit
	if more {
		rows = rows[:in.Limit]
	}

	page, err := toPage(ctx, models.New(tx), in.Membership.Member.ID, rows)
	page.More = more

	if len(rows) > 0 {
		page.Last = new(positionOf(rows[len(rows)-1], in.Sort))
	}

	return page, err
}

/*
canList refuses a caller who may not read the list.
*/
func canList(m authz.Membership, f Filters) error {
	if err := CanRead(m); err != nil {
		return err
	}

	if f.View == ViewDeleted && !m.Can("conversations:delete") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
selectConversations runs the list and returns its rows, in its order.
*/
func selectConversations(
	ctx context.Context, tx *database.Tx, list querybuilder.Conversations,
) ([]models.Conversation, error) {
	found, err := selectIDs(ctx, tx, list)
	if err != nil || len(found) == 0 {
		return nil, err
	}

	convs, err := models.New(tx).ConversationsByIDs(ctx, models.ConversationsByIDsParams{
		WorkspaceID: list.WorkspaceID, Ids: found,
	})
	if err != nil {
		return nil, err
	}

	byConversation := byID(convs, func(c models.Conversation) ids.UUID { return c.ID })
	ordered := make([]models.Conversation, 0, len(found))

	for _, id := range found {
		if c, ok := byConversation[id]; ok {
			ordered = append(ordered, c)
		}
	}

	return ordered, nil
}

/*
selectIDs runs the list and returns its ids, in its order.
*/
func selectIDs(ctx context.Context, tx *database.Tx, list querybuilder.Conversations) ([]ids.UUID, error) {
	sql, args := list.IDs()

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []ids.UUID

	for rows.Next() {
		var id ids.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		found = append(found, id)
	}

	return found, rows.Err()
}

/*
positionOf returns where the conversation sits in the order.
*/
func positionOf(c models.Conversation, s querybuilder.Sort) querybuilder.Keyset {
	return querybuilder.Keyset{
		Priority: c.Priority, ID: c.ID,
		At: s.TimeOf(c.LastMessageAt, c.WaitingSince, &c.CreatedAt, c.SnoozedUntil),
	}
}

/*
toPage loads the rows of a page.
*/
func toPage(
	ctx context.Context, q *models.Queries, memberID ids.UUID, rows []models.Conversation,
) (ConversationPage, error) {
	var page ConversationPage

	loaded, err := Load(ctx, q, memberID, rows)
	if err != nil {
		return page, err
	}

	page.Conversations = loaded

	return page, nil
}
