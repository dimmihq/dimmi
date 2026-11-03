/*
|--------------------------------------------------------------------------
| Manage Inboxes
|--------------------------------------------------------------------------
|
| Those who manage inboxes add and rename them. A new inbox comes with
| its API channel, so conversations can be started in it through the
| REST API from the start.
|
*/

package inboxes

import (
	"context"
	"encoding/json"
	"fmt"

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
CreateInboxInput is who adds an inbox and its name.
*/
type CreateInboxInput struct {
	Membership authz.Membership
	Name       string
}

/*
CreateInbox adds an inbox with its API channel.

Rules:
  - The caller needs inboxes:manage.

Events: [appevents.InboxCreated].

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type CreateInbox struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the inbox and returns it.
*/
func (a *CreateInbox) Handle(ctx context.Context, in CreateInboxInput) (models.Inbox, error) {
	if !in.Membership.Can("inboxes:manage") {
		return models.Inbox{}, exceptions.ErrForbidden
	}

	var out models.Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, now, ws := models.New(tx), a.Clock.Now(), in.Membership.Workspace.ID

		inbox, err := q.CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: ws, Name: in.Name, Settings: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create inbox: %w", err)
		}

		_, err = q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: ws, InboxID: inbox.ID, Type: "api", Name: "API", Status: "active",
			Config: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create api channel: %w", err)
		}

		out = inbox

		return a.Events.Dispatch(ctx, tx, appevents.InboxCreated{
			WorkspaceID: ws, InboxID: inbox.ID, ActorID: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
UpdateInboxInput is who renames which inbox, and its new name, nil to
keep it.
*/
type UpdateInboxInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
	Name       *string
}

/*
UpdateInbox renames an inbox.

Rules:
  - The caller needs inboxes:manage.
  - The name it already has changes nothing.

Events: [appevents.InboxUpdated] when the name changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type UpdateInbox struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle renames the inbox and returns it as it now is.
*/
func (a *UpdateInbox) Handle(ctx context.Context, in UpdateInboxInput) (models.Inbox, error) {
	if !in.Membership.Can("inboxes:manage") {
		return models.Inbox{}, exceptions.ErrForbidden
	}

	var out models.Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inbox, err := find(ctx, q, in.Membership.Workspace.ID, in.InboxID)
		if err != nil || in.Name == nil || *in.Name == inbox.Name {
			out = inbox

			return err
		}

		out, err = q.UpdateInbox(ctx, models.UpdateInboxParams{
			Name: *in.Name, Settings: inbox.Settings, Now: a.Clock.Now(), WorkspaceID: inbox.WorkspaceID, ID: inbox.ID,
		})
		if err != nil {
			return fmt.Errorf("update inbox: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.InboxUpdated{
			WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, ActorID: in.Membership.Member.ID,
			FromName: inbox.Name, ToName: out.Name,
		})
	})

	return out, err
}
