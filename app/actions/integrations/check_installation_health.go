/*
|--------------------------------------------------------------------------
| Check Installation Health
|--------------------------------------------------------------------------
|
| Every installation is asked about once per health interval whether its
| token still works. A refusal marks it broken, and teammates hear it
| once; an answer brings a broken installation back and refreshes its
| name and provider facts, since people rename their Slack workspaces. A
| provider that does not answer changes nothing but the time of the
| check, so the sweep moves on.
|
*/

package integrations

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
healthBatch is how many installations one pass of the sweep reads.
*/
const healthBatch = 100

/*
CheckInstallationHealth checks installations with their providers.

Errors: a wrapped error when a provider could not be asked or the
result not stored.
*/
type CheckInstallationHealth struct {
	*Services
}

/*
Due checks every installation not checked within the health interval.
*/
func (a *CheckInstallationHealth) Due(ctx context.Context) error {
	var errs []error

	seen := map[ids.UUID]bool{}

	for {
		due, err := models.New(a.DB).ListIntegrationInstallationsDue(ctx, models.ListIntegrationInstallationsDueParams{
			CheckedBefore: a.Clock.Now().Add(-a.Config.HealthInterval), MaxRows: healthBatch,
		})
		if err != nil {
			return fmt.Errorf("list integration installations due: %w", err)
		}

		repeated := false

		for _, row := range due {
			repeated = repeated || seen[row.IntegrationInstallationID]
			seen[row.IntegrationInstallationID] = true
			errs = append(errs, a.Handle(ctx, row.WorkspaceID, row.IntegrationInstallationID))
		}

		if len(due) < healthBatch || repeated {
			return errors.Join(errs...)
		}
	}
}

/*
Handle checks one installation.
*/
func (a *CheckInstallationHealth) Handle(ctx context.Context, workspaceID, id ids.UUID) error {
	var row models.IntegrationInstallation

	err := a.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		var err error
		row, err = findInstallation(ctx, q, workspaceID, id)

		return err
	})
	if errors.Is(err, exceptions.ErrNotFound) || row.Status == StatusDisconnected {
		return nil
	}

	if err != nil {
		return err
	}

	p, ok := a.Registry.Provider(row.Provider)
	if !ok || p.Check == nil {
		return a.touch(ctx, row)
	}

	creds, err := a.open(ctx, row.CredentialsEncrypted, installationAAD(row.ID))
	if err != nil {
		return err
	}

	health, err := p.Check(ctx, creds.Access)
	if errors.Is(err, oauth.ErrAuthFailed) {
		return errors.Join(a.fail(ctx, row, row.Provider+"_auth_failed",
			"The provider refused the token. Reconnect it."), a.touch(ctx, row))
	}

	if err != nil {
		return errors.Join(fmt.Errorf("check integration %s: %w", row.ID, err), a.touch(ctx, row))
	}

	return a.healthy(ctx, row, health)
}

/*
healthy stores what the provider said about a working installation.
*/
func (a *CheckInstallationHealth) healthy(ctx context.Context, row models.IntegrationInstallation,
	health oauth.Health,
) error {
	metadata, name := encodeMetadata(health.Metadata), cmp.Or(health.AccountName, row.Name)

	return a.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		_, err := q.RecordIntegrationCheck(ctx, models.RecordIntegrationCheckParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, Name: name, Metadata: metadata, Now: a.Clock.Now(),
		})

		return wrap("record integration check", err)
	})
}

/*
encodeMetadata encodes provider facts as a JSON object, empty for none.
*/
func encodeMetadata(facts map[string]any) json.RawMessage {
	raw, err := json.Marshal(facts)
	if err != nil || facts == nil {
		return json.RawMessage("{}")
	}

	return raw
}

/*
touch records that the installation was checked.
*/
func (a *CheckInstallationHealth) touch(ctx context.Context, row models.IntegrationInstallation) error {
	return a.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		return wrap("touch integration check", q.TouchIntegrationCheck(ctx, models.TouchIntegrationCheckParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, Now: a.Clock.Now(),
		}))
	})
}
