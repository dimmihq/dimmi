/*
|--------------------------------------------------------------------------
| Disconnect Installation
|--------------------------------------------------------------------------
|
| A teammate who manages integrations disconnects a tool. The tokens are
| given back to the provider first, best effort, then wiped: the
| installation is removed, every teammate's grant on it is revoked, and
| every feature that used it hears integration.disconnected, so a
| channel stops and a destination fails at once instead of on its next
| send.
|
*/

package integrations

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The reasons an installation was disconnected: by a teammate, uninstalled
at the provider, or its tokens revoked there.
*/
const (
	ReasonDisconnected = "disconnected"
	ReasonUninstalled  = "uninstalled"
	ReasonRevoked      = "revoked"
)

/*
DisconnectInstallationInput is who disconnects and the installation.
*/
type DisconnectInstallationInput struct {
	Membership     authz.Membership
	InstallationID ids.UUID
}

/*
DisconnectInstallation removes an installation.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped error.
*/
type DisconnectInstallation struct {
	*Services
}

/*
Handle gives the tokens back and removes the installation.
*/
func (a *DisconnectInstallation) Handle(ctx context.Context, in DisconnectInstallationInput) error {
	m := in.Membership
	if err := requireManage(m); err != nil {
		return err
	}

	var row models.IntegrationInstallation

	err := a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		var err error
		row, err = findInstallation(ctx, q, m.Workspace.ID, in.InstallationID)

		return err
	})
	if err != nil {
		return err
	}

	if creds, err := a.open(ctx, row.CredentialsEncrypted, installationAAD(row.ID)); err == nil {
		a.revokeQuietly(ctx, row.Provider, creds.Access)
	}

	tokens, err := a.disconnect(ctx, row, disconnection{actor: m.Member.ID, reason: ReasonDisconnected, removed: true})
	if err != nil {
		return err
	}

	for _, token := range tokens {
		a.revokeQuietly(ctx, row.Provider, token)
	}

	return nil
}

/*
disconnection is how an installation goes: who did it, ids.Nil() for
the provider, why, and whether it is removed from the workspace's list
or stays there to be reconnected.
*/
type disconnection struct {
	actor   ids.UUID
	reason  string
	removed bool
}

/*
disconnect marks the installation disconnected with its tokens wiped,
revokes its grants, announces and audits it, and returns the grants'
tokens.
*/
func (s *Services) disconnect(ctx context.Context, row models.IntegrationInstallation, d disconnection,
) ([]string, error) {
	var tokens []string

	err := s.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, tx *database.Tx) error {
		sealed, err := s.seal(ctx, row.WorkspaceID, installationAAD(row.ID), credentials{})
		if err != nil {
			return err
		}

		if _, err := q.DisconnectIntegrationInstallation(ctx, models.DisconnectIntegrationInstallationParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, CredentialsEncrypted: sealed, Now: s.Clock.Now(),
			Removed: d.removed,
		}); err != nil {
			return wrap("disconnect integration installation", err)
		}

		grants, err := activeGrants(ctx, q, row, nil)
		if err != nil {
			return err
		}

		if tokens, err = s.revokeGrants(ctx, q, grantSet{row: row, grants: grants}, d.actor); err != nil {
			return err
		}

		return s.announceDisconnect(ctx, tx, row, d)
	})

	return tokens, err
}
