/*
|--------------------------------------------------------------------------
| Inboxes
|--------------------------------------------------------------------------
|
| An inbox is where a workspace's conversations land, routed there by
| the channels connected to it. Every member sees the inboxes; adding
| and renaming them is for those who manage inboxes. A new inbox comes
| with its API channel, so conversations can be started in it through
| the REST API from the start.
|
*/

package inboxes

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListInboxes lists the workspace's live inboxes, oldest first.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListInboxes struct {
	DB *database.DB
}

/*
Handle returns the inboxes.
*/
func (a *ListInboxes) Handle(ctx context.Context, m authz.Membership) ([]models.Inbox, error) {
	if !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = models.New(tx).ListInboxes(ctx, m.Workspace.ID)

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list inboxes: %w", err)
	}

	return out, nil
}

/*
ShowInboxInput is who asks about which inbox.
*/
type ShowInboxInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
}

/*
ShowInbox returns one live inbox of the workspace.

Rules:
  - The caller needs conversations:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowInbox struct {
	DB *database.DB
}

/*
Handle returns the inbox.
*/
func (a *ShowInbox) Handle(ctx context.Context, in ShowInboxInput) (models.Inbox, error) {
	if !in.Membership.Can("conversations:read") {
		return models.Inbox{}, exceptions.ErrForbidden
	}

	var out models.Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = find(ctx, models.New(tx), in.Membership.Workspace.ID, in.InboxID)

		return err
	})

	return out, err
}

/*
find returns the live inbox, or exceptions.ErrNotFound.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Inbox, error) {
	inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
		return inbox, exceptions.ErrNotFound
	}

	if err != nil {
		return inbox, fmt.Errorf("find inbox: %w", err)
	}

	return inbox, nil
}
