/*
|--------------------------------------------------------------------------
| List Conversations
|--------------------------------------------------------------------------
|
| The inbox's lists: the caller's own conversations, those nobody holds,
| everyone's, one teammate's, and spam, each narrowed by status, inbox,
| customer, priority, unread state and a search, newest activity first
| or in another order. Pages follow each other by the last row's sort
| key and id, so a conversation arriving while someone pages never makes
| a row repeat or vanish.
|
| Spam never shows outside the spam view. Mentions have their own view,
| which stays empty until mentions are recorded.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListConversationsInput is who asks and which page of which list.
*/
type ListConversationsInput struct {
	Membership authz.Membership
	Filters    Filters
	Sort       Sort
	After      *Cursor
	Limit      int
}

/*
ConversationPage is a page of conversations, the cursor of its last row
and whether more follow.
*/
type ConversationPage struct {
	Conversations []Loaded
	Last          *Cursor
	More          bool
}

/*
ListConversations lists a page of the workspace's conversations.

Rules:
  - The caller needs conversations:read.
  - Deleted conversations never show; spam shows only in the spam view.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListConversations struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListConversations) Handle(ctx context.Context, in ListConversationsInput) (ConversationPage, error) {
	if err := CanRead(in.Membership); err != nil {
		return ConversationPage{}, err
	}

	params, ok := listParams(in.Membership, in.Filters, in.Sort)
	if !ok {
		return ConversationPage{}, nil
	}

	params.RowLimit = int64(in.Limit) + 1
	if in.After != nil {
		params.AfterAt, params.AfterID = &in.After.At, &in.After.ID
	}

	var page ConversationPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.FilterConversations(ctx, params)
		if err != nil {
			return fmt.Errorf("list conversations: %w", err)
		}

		more := len(rows) > in.Limit
		if more {
			rows = rows[:in.Limit]
		}

		page, err = toPage(ctx, q, in.Membership.Member.ID, rows)
		page.More = more

		return err
	})

	return page, err
}

/*
toPage loads the rows of a page.
*/
func toPage(
	ctx context.Context, q *models.Queries, memberID ids.UUID, rows []models.FilterConversationsRow,
) (ConversationPage, error) {
	var page ConversationPage

	convs := make([]models.Conversation, len(rows))
	for i, row := range rows {
		convs[i] = row.Conversation
	}

	loaded, err := Load(ctx, q, memberID, convs)
	if err != nil {
		return page, err
	}

	page.Conversations = loaded
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		page.Last = &Cursor{At: last.SortAt, ID: last.Conversation.ID}
	}

	return page, nil
}
