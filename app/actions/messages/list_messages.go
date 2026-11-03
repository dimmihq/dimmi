/*
|--------------------------------------------------------------------------
| List Messages
|--------------------------------------------------------------------------
|
| A conversation's timeline, read a page at a time: the latest page
| first, then older pages walking back, or newer ones walking forward
| from where a reader stopped. Pages always read oldest first. Messages,
| notes and events can be asked for alone or together. Each page comes
| with the people who wrote it, so every entry can say who it is from.
|
*/

package messages

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Cursor is where a page starts: next to the entry with the time and id.
*/
type Cursor struct {
	At time.Time
	ID ids.UUID
}

/*
ListMessagesInput is who reads which conversation's timeline, and which
page: before the entry Before, after the entry After, or the latest
page when neither. Types narrows to entry types, none meaning all.
*/
type ListMessagesInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	Before     *Cursor
	After      *Cursor
	Types      []string
	Limit      int
}

/*
Timeline is a page of entries, oldest first, with the conversation's
customer and the members who wrote them. Older and Newer say whether
entries lie beyond the page on either side; a page read backwards does
not look forward, and the other way round.
*/
type Timeline struct {
	Messages []models.Message
	Contact  models.Contact
	Members  map[ids.UUID]conversations.Member
	Older    bool
	Newer    bool
}

/*
ListMessages reads a page of a conversation's timeline.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListMessages struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListMessages) Handle(ctx context.Context, in ListMessagesInput) (Timeline, error) {
	if err := conversations.CanRead(in.Membership); err != nil {
		return Timeline{}, err
	}

	var out Timeline

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := conversations.Find(ctx, q, in.Membership.Workspace.ID, in.Ref)
		if err != nil {
			return err
		}

		if out, err = page(ctx, q, c, in); err != nil {
			return err
		}

		return out.loadPeople(ctx, q, c)
	})

	return out, err
}

/*
page reads the entries of the page, putting them oldest first.
*/
func page(ctx context.Context, q *models.Queries, c models.Conversation, in ListMessagesInput) (Timeline, error) {
	params := models.ListTimelineParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Types: in.Types, Descending: in.After == nil,
		RowLimit: int64(in.Limit) + 1,
	}

	cursor := in.Before
	if in.After != nil {
		cursor = in.After
	}

	if cursor != nil {
		params.CursorAt, params.CursorID = &cursor.At, &cursor.ID
	}

	rows, err := q.ListTimeline(ctx, params)
	if err != nil {
		return Timeline{}, fmt.Errorf("list messages: %w", err)
	}

	more := len(rows) > in.Limit
	if more {
		rows = rows[:in.Limit]
	}

	out := Timeline{Messages: rows, Older: more && params.Descending, Newer: more && !params.Descending}
	if params.Descending {
		slices.Reverse(out.Messages)
	}

	return out, nil
}

/*
loadPeople reads the conversation's customer and the members who wrote
the page.
*/
func (t *Timeline) loadPeople(ctx context.Context, q *models.Queries, c models.Conversation) error {
	contact, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: c.WorkspaceID, ID: c.ContactID})
	if err != nil {
		return fmt.Errorf("find contact: %w", err)
	}

	var memberIDs []ids.UUID

	for _, m := range t.Messages {
		if m.AuthorType == string(enums.AuthorTypeMember) && m.AuthorID != nil {
			memberIDs = append(memberIDs, *m.AuthorID)
		}
	}

	members, err := conversations.Members(ctx, q, c.WorkspaceID, memberIDs)
	if err != nil {
		return fmt.Errorf("load authors: %w", err)
	}

	t.Contact, t.Members = contact, members

	return nil
}
