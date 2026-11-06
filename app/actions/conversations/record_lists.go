/*
|--------------------------------------------------------------------------
| Conversations Of A Record
|--------------------------------------------------------------------------
|
| A customer's or a company's past conversations, newest first, whatever
| their status. They go through the same list as every other, so anything
| the reader may not see in the inbox stays out here too. Spam and the
| trash do not count as history.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/querybuilder"
)

/*
RecordConversationsInput is who asks for which page of the conversations
of a contact or a company. Except leaves one conversation out, nil for
none.
*/
type RecordConversationsInput struct {
	Membership authz.Membership
	ContactID  *ids.UUID
	CompanyID  *ids.UUID
	After      *querybuilder.Keyset
	Limit      int
}

/*
ListRecordConversations lists the conversations of a contact or a
company.

Rules:
  - The caller needs conversations:read.
  - Conversations outside what the caller may see stay out.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListRecordConversations struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListRecordConversations) Handle(ctx context.Context, in RecordConversationsInput) (ConversationPage, error) {
	if err := CanRead(in.Membership); err != nil {
		return ConversationPage{}, err
	}

	var page ConversationPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		list, err := recordList(ctx, q, in)
		if err != nil {
			return err
		}

		rows, err := selectConversations(ctx, tx, list)
		if err != nil {
			return fmt.Errorf("list record conversations: %w", err)
		}

		more := len(rows) > in.Limit
		if more {
			rows = rows[:in.Limit]
		}

		page, err = toPage(ctx, q, in.Membership.Member.ID, rows)
		page.More = more

		if len(rows) > 0 {
			page.Last = new(positionOf(rows[len(rows)-1], querybuilder.SortLatest))
		}

		return err
	})

	return page, err
}

/*
recordList narrows the list to the record, following a merged contact
to the one it was merged into.
*/
func recordList(
	ctx context.Context, q *models.Queries, in RecordConversationsInput,
) (querybuilder.Conversations, error) {
	m := in.Membership
	list, _ := listOf(m, Filters{View: ViewAll, AnyStatus: true}, querybuilder.SortLatest)
	list.Limit, list.After = in.Limit+1, in.After

	if in.ContactID != nil {
		contact, err := contacts.Survivor(ctx, q, m.Workspace.ID, *in.ContactID)
		if err != nil {
			return list, err
		}

		list.ContactID = &contact.ID

		return list, nil
	}

	if in.CompanyID == nil {
		return list, exceptions.ErrNotFound
	}

	company, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: m.Workspace.ID, ID: *in.CompanyID})
	if errors.Is(err, database.ErrNotFound) || (err == nil && company.DeletedAt != nil) {
		return list, exceptions.ErrNotFound
	}

	if err != nil {
		return list, fmt.Errorf("find company: %w", err)
	}

	list.CompanyID = &company.ID

	return list, nil
}
