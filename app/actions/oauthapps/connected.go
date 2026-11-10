/*
|--------------------------------------------------------------------------
| Connected Apps
|--------------------------------------------------------------------------
|
| A person sees every app they connected: which app, the workspace it
| acts in, the access it holds and when it last renewed its access, and
| can disconnect any of them, which ends its access at once. Nobody else
| sees or ends a person's connections from here.
|
*/

package oauthapps

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauth"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
App is one connected app with the name of the workspace it acts in.
*/
type App struct {
	oauth.Grant
	WorkspaceID   ids.UUID
	WorkspaceName string
}

/*
Connected lists and disconnects the apps a person connected.

Errors: [exceptions.ErrNotFound] for an app that is not the person's, or
a wrapped database error.
*/
type Connected struct {
	DB     *database.DB
	Server *oauth.Server
}

/*
List returns the person's connected apps, newest first.
*/
func (a *Connected) List(ctx context.Context, user ids.UUID) ([]App, error) {
	grants, err := a.Server.Grants(ctx, user)
	if err != nil {
		return nil, err
	}

	rows, err := models.New(a.DB).ListUserActiveMemberships(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}

	names := map[string]models.ListUserActiveMembershipsRow{}
	for _, row := range rows {
		names[row.WorkspaceID.String()] = row
	}

	out := make([]App, 0, len(grants))
	for _, g := range grants {
		app := App{Grant: g}
		if row, ok := names[g.Tenant]; ok {
			app.WorkspaceID, app.WorkspaceName = row.WorkspaceID, row.WorkspaceName
		}

		out = append(out, app)
	}

	return out, nil
}

/*
Disconnect ends one of the person's connected apps.
*/
func (a *Connected) Disconnect(ctx context.Context, user, id ids.UUID) error {
	err := a.Server.RevokeUserGrant(ctx, user, id)
	if errors.Is(err, oauth.ErrGrantNotFound) {
		return exceptions.ErrNotFound
	}

	return err
}
