/*
|--------------------------------------------------------------------------
| The Trash
|--------------------------------------------------------------------------
|
| Deleting a conversation moves it to the trash: it leaves every list
| and count but keeps its messages, so it can be put back. Both take
| conversations:delete, the permission that shows the trash.
|
*/

package conversations

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TrashInput is who moves which conversation.
*/
type TrashInput struct {
	Membership authz.Membership
	Ref        Ref
	MutationID string
}

/*
TrashConversation moves a conversation to the trash.

Rules:
  - The caller needs conversations:delete and must see the conversation.

Events: [appevents.ConversationDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type TrashConversation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle moves the conversation to the trash.
*/
func (a *TrashConversation) Handle(ctx context.Context, in TrashInput) error {
	if !in.Membership.Can("conversations:delete") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := NewWriter(ctx, tx, Services{Events: a.Events, Clock: a.Clock}, in.Membership)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		c, err := Lock(ctx, w.Q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		trashed, err := w.Q.TrashConversation(ctx, models.TrashConversationParams{
			Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
		})
		if err != nil {
			return missing(err)
		}

		return w.Dispatch(ctx, appevents.ConversationDeleted{
			Base: w.Base(c.WorkspaceID), ConversationRef: ref(trashed),
		})
	})
}

/*
RestoreConversation takes a conversation out of the trash.

Rules:
  - The caller needs conversations:delete and must see the conversation.
  - A conversation that is not in the trash is not found.

Events: [appevents.ConversationUpdated], naming deleted.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type RestoreConversation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle puts the conversation back and returns it as it now is.
*/
func (a *RestoreConversation) Handle(ctx context.Context, in TrashInput) (Loaded, error) {
	if !in.Membership.Can("conversations:delete") {
		return Loaded{}, exceptions.ErrForbidden
	}

	var out Loaded

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w, err := NewWriter(ctx, tx, Services{Events: a.Events, Clock: a.Clock}, in.Membership)
		if err != nil {
			return err
		}

		w.MutationID = in.MutationID

		c, err := findTrashed(ctx, w.Q, in.Membership, in.Ref)
		if err != nil {
			return err
		}

		restored, err := w.Q.RestoreConversation(ctx, models.RestoreConversationParams{
			Now: w.Now, WorkspaceID: c.WorkspaceID, ID: c.ID,
		})
		if err != nil {
			return missing(err)
		}

		err = w.Dispatch(ctx, appevents.ConversationUpdated{
			Base: w.Base(c.WorkspaceID), ConversationRef: ref(restored),
			Changes: map[string][2]any{"deleted": {true, false}},
		})
		if err != nil {
			return err
		}

		out, err = LoadOne(ctx, w.Q, in.Membership.Member.ID, restored)

		return err
	})

	return out, err
}

/*
findTrashed returns the trashed conversation the reference names, when
the caller may see it.
*/
func findTrashed(ctx context.Context, q *models.Queries, m authz.Membership, r Ref) (models.Conversation, error) {
	var (
		c   models.Conversation
		err error
	)

	if r.ID == ids.Nil() {
		c, err = q.FindConversationByNumber(ctx, models.FindConversationByNumberParams{
			WorkspaceID: m.Workspace.ID, Number: r.Number,
		})
	} else {
		c, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: m.Workspace.ID, ID: r.ID})
	}

	if err != nil {
		return c, missing(err)
	}

	if c.DeletedAt == nil {
		return c, exceptions.ErrNotFound
	}

	reads, err := authz.Reads(ctx, q, VisibleScope(m), c)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return c, err
	}

	if !reads {
		return c, exceptions.ErrNotFound
	}

	return c, nil
}
