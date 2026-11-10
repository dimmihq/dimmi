/*
|--------------------------------------------------------------------------
| Apply Provider Revocation
|--------------------------------------------------------------------------
|
| The provider wins: when it reports that the app was uninstalled or its
| token revoked, the installation is disconnected at once, its grants go
| with it, and every feature hears integration.disconnected. When it
| reports that some teammates revoked their own tokens, only their
| grants go, and the app keeps working as itself. The installation stays
| in the workspace's list, disconnected, so a teammate can reconnect it.
|
| The report names only the provider's own app and account ids, so the
| installation is found across every workspace.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Revocation is what a provider reported: for which app and account, the
reason, uninstalled or revoked, whether the app's own token went, and
the provider users whose own tokens went.
*/
type Revocation struct {
	Provider  string
	AppID     string
	AccountID string
	Reason    string
	Bot       bool
	UserIDs   []string
}

/*
ApplyProviderRevocation applies what a provider reported. A report about
an installation that is not live changes nothing.

Errors: a wrapped error.
*/
type ApplyProviderRevocation struct {
	*Services
}

/*
Handle applies the report.
*/
func (a *ApplyProviderRevocation) Handle(ctx context.Context, r Revocation) error {
	route, err := models.New(a.DB).RouteIntegrationInstallation(ctx, models.RouteIntegrationInstallationParams{
		Provider: r.Provider, AppID: r.AppID, AccountID: r.AccountID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("route integration installation: %w", err)
	}

	var row models.IntegrationInstallation

	err = a.inWorkspace(ctx, route.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		row, err = findInstallation(ctx, q, route.WorkspaceID, route.IntegrationInstallationID)

		return err
	})
	if err != nil {
		return err
	}

	if r.Bot {
		_, err := a.disconnect(ctx, row, disconnection{actor: ids.Nil(), reason: r.Reason})

		return err
	}

	return a.revokeUsers(ctx, row, r.UserIDs)
}

/*
revokeUsers revokes the grants of the provider users.
*/
func (a *ApplyProviderRevocation) revokeUsers(ctx context.Context, row models.IntegrationInstallation,
	users []string,
) error {
	if len(users) == 0 {
		return nil
	}

	return a.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		grants, err := activeGrants(ctx, q, row, users)
		if err != nil {
			return err
		}

		_, err = a.revokeGrants(ctx, q, grantSet{row: row, grants: grants}, ids.Nil())

		return err
	})
}

/*
announceDisconnect dispatches integration.disconnected and audits it.
*/
func (s *Services) announceDisconnect(ctx context.Context, tx *database.Tx, row models.IntegrationInstallation,
	d disconnection,
) error {
	err := s.Events.Dispatch(ctx, tx, appevents.IntegrationDisconnected{
		Base: s.base(row.WorkspaceID, d.actor), InstallationID: row.ID, Provider: row.Provider, Reason: d.reason,
	})
	if err != nil {
		return fmt.Errorf("dispatch integration disconnected: %w", err)
	}

	return s.record(ctx, row, note{actor: d.actor, event: AuditDisconnected, changes: map[string]any{
		"provider": row.Provider, "account": deref(row.ExternalAccountID), "reason": d.reason,
	}})
}
