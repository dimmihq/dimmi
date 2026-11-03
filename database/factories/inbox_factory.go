/*
|--------------------------------------------------------------------------
| Inbox Factory
|--------------------------------------------------------------------------
|
| Inboxes and channel connections for tests, written through the same
| queries the product uses, inside the workspace they belong to. An
| inbox comes with its API channel, as one made through the API does.
|
*/

package factories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
InboxWithChannel is an inbox and its API channel.
*/
type InboxWithChannel struct {
	Inbox models.Inbox
	API   models.ChannelConnection
}

/*
Inbox creates an inbox with the name and its API channel in the
workspace.
*/
func Inbox(ctx context.Context, a *app.App, workspace models.Workspace, name string) (InboxWithChannel, error) {
	var out InboxWithChannel

	scoped := tenancy.With(ctx, workspace.ID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		out.Inbox, err = q.CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: workspace.ID, Name: name, Settings: json.RawMessage(`{}`), Now: a.Clock.Now(),
		})
		if err != nil {
			return err
		}

		out.API, err = q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: workspace.ID, InboxID: out.Inbox.ID, Type: "api", Name: "API", Status: "active",
			Config: json.RawMessage(`{}`), Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return out, fmt.Errorf("create inbox: %w", err)
	}

	return out, nil
}

/*
Connection connects a channel of the type to the inbox.
*/
func Connection(
	ctx context.Context, a *app.App, inbox models.Inbox, channelType string,
) (models.ChannelConnection, error) {
	var out models.ChannelConnection

	scoped := tenancy.With(ctx, inbox.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = models.New(tx).CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID, Type: channelType, Name: channelType,
			Status: "active", Config: json.RawMessage(`{}`), Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return out, fmt.Errorf("create channel connection: %w", err)
	}

	return out, nil
}
