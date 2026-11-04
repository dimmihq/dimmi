/*
|--------------------------------------------------------------------------
| Ensure Default Inbox
|--------------------------------------------------------------------------
|
| Every workspace has a default inbox from the moment it exists, so a
| conversation always has somewhere to land, and every live inbox has an
| API connection, so conversations can be started through the API in any
| of them. Both are created while the workspace is, and created again by
| a repair when someone removed them, never lazily on first use.
|
*/

package inboxes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The names of the initializer and of the defaults it creates.
*/
const (
	initializerName   = "default_inbox"
	defaultInboxName  = "Support"
	apiConnectionName = "API"
)

/*
EnsureDefaultInbox is the workspace initializer that gives a workspace
its default inbox and every live inbox its API connection.

Rules:
  - A workspace whose settings name no live inbox as default gets a new
    "Support" inbox, named in the settings.
  - Every live inbox without a live API connection gets an active one.
  - Running it on a workspace that has both changes nothing.
*/
type EnsureDefaultInbox struct {
	Clock clock.Clock
}

/*
Name returns "default_inbox".
*/
func (a *EnsureDefaultInbox) Name() string {
	return initializerName
}

/*
Initialize creates the default inbox and the API connections the
workspace lacks, in the transaction.
*/
func (a *EnsureDefaultInbox) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	q := models.New(tx)

	_, err := q.FindDefaultInbox(ctx, w.ID)
	if errors.Is(err, database.ErrNotFound) {
		err = a.createDefault(ctx, q, w)
	}

	if err != nil {
		return fmt.Errorf("ensure default inbox: %w", err)
	}

	return a.connectAPI(ctx, q, w)
}

/*
createDefault creates a "Support" inbox and makes it the workspace's
default.
*/
func (a *EnsureDefaultInbox) createDefault(ctx context.Context, q *models.Queries, w models.Workspace) error {
	now := a.Clock.Now()

	inbox, err := q.CreateInbox(ctx, models.CreateInboxParams{
		WorkspaceID: w.ID, Name: defaultInboxName, Settings: json.RawMessage(`{}`), Now: now,
	})
	if err != nil {
		return fmt.Errorf("create inbox: %w", err)
	}

	return q.SetDefaultInbox(ctx, models.SetDefaultInboxParams{WorkspaceID: w.ID, InboxID: inbox.ID, Now: now})
}

/*
connectAPI gives every live inbox of the workspace without one an
active API connection.
*/
func (a *EnsureDefaultInbox) connectAPI(ctx context.Context, q *models.Queries, w models.Workspace) error {
	inboxes, err := q.ListInboxesWithoutChannel(ctx, models.ListInboxesWithoutChannelParams{
		WorkspaceID: w.ID, Type: string(enums.ChannelTypeAPI),
	})
	if err != nil {
		return fmt.Errorf("list inboxes without an api connection: %w", err)
	}

	for _, inbox := range inboxes {
		_, err := q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: w.ID, InboxID: inbox.ID, Type: string(enums.ChannelTypeAPI), Name: apiConnectionName,
			Status: "active", Config: json.RawMessage(`{}`), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("connect the api to inbox %s: %w", inbox.ID, err)
		}
	}

	return nil
}
