/*
|--------------------------------------------------------------------------
| Inboxes
|--------------------------------------------------------------------------
|
| An inbox is where a workspace's conversations land, routed there by
| the channels connected to it. Members see the inboxes they may open; a
| restricted inbox stays out of sight for everyone not on its list.
| Adding and renaming them is for those who manage inboxes. A new inbox comes
| with its API channel, so conversations can be started in it through
| the REST API from the start.
|
*/

package inboxes

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Inbox is an inbox, whether it is the workspace's default, where
conversations land when nothing routes them elsewhere, and who may open
it.
*/
type Inbox struct {
	models.Inbox
	Default bool
	Access  Access
}

/*
ListInboxes lists the workspace's live inboxes the caller may open,
oldest first.

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
func (a *ListInboxes) Handle(ctx context.Context, m authz.Membership) ([]Inbox, error) {
	if !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListInboxes(ctx, m.Workspace.ID)
		if err != nil {
			return err
		}

		def, err := defaultID(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}

		access, err := accessOf(ctx, q, m.Workspace.ID, rows)
		if err != nil {
			return err
		}

		out = []Inbox{}
		for _, row := range rows {
			if slices.Contains(m.HiddenInboxes, row.ID) {
				continue
			}

			out = append(out, Inbox{Inbox: row, Default: row.ID == def, Access: access[row.ID]})
		}

		return nil
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
ShowInbox returns one live inbox of the workspace the caller may open.

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
func (a *ShowInbox) Handle(ctx context.Context, in ShowInboxInput) (Inbox, error) {
	if !in.Membership.Can("conversations:read") {
		return Inbox{}, exceptions.ErrForbidden
	}

	var out Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		if slices.Contains(in.Membership.HiddenInboxes, in.InboxID) {
			return exceptions.ErrNotFound
		}

		inbox, err := find(ctx, q, in.Membership.Workspace.ID, in.InboxID)
		if err != nil {
			return err
		}

		out, err = withDefault(ctx, q, inbox)

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

/*
defaultID returns the id of the workspace's default inbox, the nil id
when it has none.
*/
func defaultID(ctx context.Context, q *models.Queries, ws ids.UUID) (ids.UUID, error) {
	def, err := q.FindDefaultInbox(ctx, ws)
	if errors.Is(err, database.ErrNotFound) {
		return ids.Nil(), nil
	}

	if err != nil {
		return ids.Nil(), fmt.Errorf("find default inbox: %w", err)
	}

	return def.ID, nil
}

/*
withDefault returns the inbox with whether it is the default and who may
open it.
*/
func withDefault(ctx context.Context, q *models.Queries, inbox models.Inbox) (Inbox, error) {
	def, err := defaultID(ctx, q, inbox.WorkspaceID)
	if err != nil {
		return Inbox{}, err
	}

	access, err := accessOf(ctx, q, inbox.WorkspaceID, []models.Inbox{inbox})

	return Inbox{Inbox: inbox, Default: inbox.ID == def, Access: access[inbox.ID]}, err
}
