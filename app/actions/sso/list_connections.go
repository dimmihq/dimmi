/*
|--------------------------------------------------------------------------
| List And Delete Connections
|--------------------------------------------------------------------------
|
| Owners see the organization's connections with their settings, never
| their secrets. A connection goes only once it no longer binds anyone:
| deleting an enforced one would leave everyone it governs at a sign-in
| page that no longer leads anywhere. Identities linked through it go
| with it, and their people fall back to their other ways in.
|
*/

package sso

import (
	"context"
	"errors"
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
ListConnections returns the connections of the organization of the
workspace in play.

Rules:
  - The caller needs sso:manage.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListConnections struct {
	DB *database.DB
}

/*
Handle returns the connections, oldest first.
*/
func (a *ListConnections) Handle(ctx context.Context, m authz.Membership) ([]Connection, error) {
	if !m.Can(managePermission) {
		return nil, exceptions.ErrForbidden
	}

	rows, err := models.New(a.DB).ListSSOConnections(ctx, m.Workspace.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}

	out := make([]Connection, 0, len(rows))

	for _, row := range rows {
		conn, err := Decode(row)
		if err != nil {
			return nil, err
		}

		out = append(out, conn)
	}

	return out, nil
}

/*
ShowConnection returns one connection of the organization of the
workspace in play.

Rules:
  - The caller needs sso:manage.
  - Connections of other organizations are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowConnection struct {
	DB *database.DB
}

/*
Handle returns the connection.
*/
func (a *ShowConnection) Handle(ctx context.Context, m authz.Membership, id ids.UUID) (Connection, error) {
	if !m.Can(managePermission) {
		return Connection{}, exceptions.ErrForbidden
	}

	row, err := models.New(a.DB).FindSSOConnection(ctx, models.FindSSOConnectionParams{
		OrganizationID: m.Workspace.OrganizationID, ID: id,
	})
	if errors.Is(err, database.ErrNotFound) {
		return Connection{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Connection{}, fmt.Errorf("find connection: %w", err)
	}

	return Decode(row)
}

/*
DeleteConnection removes a connection of the organization of the
workspace in play.

Rules:
  - The caller needs sso:manage.
  - Connections of other organizations are not found.
  - An enforced connection is refused.

Events: [appevents.SSOConnectionChanged] deleted.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrConnectionEnforced], or a wrapped database error.
*/
type DeleteConnection struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle deletes the connection.
*/
func (a *DeleteConnection) Handle(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can(managePermission) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		organizationID := m.Workspace.OrganizationID

		row, err := lockConnection(ctx, q, organizationID, id)
		if err != nil {
			return err
		}

		if row.Enforced {
			return exceptions.ErrConnectionEnforced
		}

		if _, err := q.DeleteSSOConnection(ctx, models.DeleteSSOConnectionParams{
			OrganizationID: organizationID, ID: id,
		}); err != nil {
			return fmt.Errorf("delete connection: %w", err)
		}

		if err := Mirror(ctx, q, organizationID, a.Clock.Now()); err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.SSOConnectionChanged{
			WorkspaceID: m.Workspace.ID, ConnectionID: row.ID, Name: row.Name, Change: "deleted",
			ChangedBy: m.Member.ID,
		})
	})
}
