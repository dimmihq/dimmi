/*
|--------------------------------------------------------------------------
| Connected Accounts
|--------------------------------------------------------------------------
|
| A source of a service connected over OAuth, such as Notion, Intercom
| or Zendesk, names the workspace's installation of that service in its
| "installation" setting. Every time its driver runs, the engine asks
| the integrations for a working token, renewed when it expired, and
| hands it to the driver with the account it reaches, so no key is ever
| typed into a form or stored on the source. Reconnecting an account
| renews the same installation; a source still naming an older copy of
| that account follows to the live one. A disconnected or broken
| installation reads as refused credentials, so the source shows it.
|
*/

package knowledgesources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
installationPrefix is the public id prefix of integration installations.
*/
const installationPrefix = "itg"

/*
Connections hands out working tokens of the workspace's installations.
*/
type Connections interface {
	/*
		Bot returns a working app token of the installation.
	*/
	Bot(ctx context.Context, installationID ids.UUID) (string, error)
}

/*
connect adds the token and account of the source's installation to its
settings, for a driver that reads through a connected service.
*/
func (e *Engine) connect(ctx context.Context, ws ids.UUID, d sources.Driver, merged map[string]any) error {
	provider := d.Describe().Integration
	if provider == "" {
		return nil
	}

	public, _ := merged["installation"].(string)

	id, err := e.IDs.Decode(installationPrefix, public)
	if err != nil || e.Connections == nil {
		return &sources.ConfigError{Field: "installation", Message: "Connect your account first."}
	}

	var row models.IntegrationInstallation

	err = e.inWorkspace(ctx, ws, func(ctx context.Context, _ *database.Tx, q *models.Queries) error {
		row, err = q.FindIntegrationInstallation(ctx, models.FindIntegrationInstallationParams{WorkspaceID: ws, ID: id})
		if err != nil || row.Status != "disconnected" {
			return err
		}

		row, err = q.FindIntegrationInstallationByAccount(ctx, models.FindIntegrationInstallationByAccountParams{
			WorkspaceID: ws, Provider: row.Provider, ExternalAppID: row.ExternalAppID,
			ExternalAccountID: row.ExternalAccountID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && row.Provider != provider) {
		return &sources.ConfigError{Field: "installation", Message: "That account is no longer connected."}
	}

	if err != nil {
		return fmt.Errorf("find installation: %w", err)
	}

	token, err := e.Connections.Bot(tenancy.With(ctx, ws.String()), row.ID)
	if err != nil {
		return fmt.Errorf("token of %s: %w: %w", provider, sources.ErrUnauthorized, err)
	}

	merged["access_token"], merged["account"] = token, row.ExternalAccountID

	return nil
}
