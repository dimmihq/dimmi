//go:build dev

/*
|--------------------------------------------------------------------------
| The Demo Workspace
|--------------------------------------------------------------------------
|
| How the demo commands find the Stowline workspace workspace:demo left:
| its owner, and the Support inbox with its messenger connection, where
| simulated customers write in.
|
*/

package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/demo/stowline"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
demoPlace is the demo workspace's owner and where its customers write
in.
*/
type demoPlace struct {
	Owner   authz.Membership
	Inbox   ids.UUID
	Channel ids.UUID
}

/*
demo finds the Stowline workspace, its owner, its Support inbox and
that inbox's messenger connection.
*/
func (c AISimulateCommand) demo(ctx context.Context) (demoPlace, error) {
	owner, err := demoOwner(ctx, c.App)
	if err != nil {
		return demoPlace{}, err
	}
	place := demoPlace{Owner: owner}
	err = c.App.DB.Transaction(tenancy.With(ctx, owner.Workspace.ID.String()),
		func(ctx context.Context, tx *database.Tx) error {
			return tx.QueryRow(ctx, `SELECT i.id, c.id FROM inboxes i
				JOIN channel_connections c ON c.inbox_id = i.id AND c.type = 'widget' AND c.deleted_at IS NULL
				WHERE i.workspace_id = $1 AND i.name = 'Support' AND i.deleted_at IS NULL LIMIT 1`,
				owner.Workspace.ID).Scan(&place.Inbox, &place.Channel)
		})
	if err != nil {
		return place, fmt.Errorf("find the Support inbox's messenger: %w", err)
	}
	return place, nil
}

/*
demoOwner returns the owner's membership in the Stowline workspace.
*/
func demoOwner(ctx context.Context, a *app.App) (authz.Membership, error) {
	w, found, err := stowline.Find(ctx, a)
	if err != nil {
		return authz.Membership{}, err
	}
	if !found {
		return authz.Membership{}, errors.New("no Stowline workspace; run workspace:demo first")
	}
	var member models.WorkspaceMember
	err = a.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		var id ids.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM workspace_members WHERE workspace_id = $1 AND role = 'owner'
			AND status = 'active' ORDER BY created_at LIMIT 1`, w.ID).Scan(&id); err != nil {
			return fmt.Errorf("find owner: %w", err)
		}
		member, err = models.New(tx).FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
			WorkspaceID: w.ID, ID: id,
		})
		return err
	})
	if err != nil {
		return authz.Membership{}, err
	}
	return authz.NewMembership(w, member, appauth.RolePermissions(enums.RoleOwner)), nil
}
