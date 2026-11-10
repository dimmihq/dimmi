/*
|--------------------------------------------------------------------------
| Installation Store
|--------------------------------------------------------------------------
|
| Writing an installation is the same whether its tokens came from an
| OAuth install or were pasted from a custom app: the tokens are sealed
| to the row, and a provider account another workspace already holds
| live is refused, since its events could reach only one tenant.
|
*/

package integrations

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
installation is what an install writes: the provider account and app,
its display name, where the tokens came from, the scopes granted, the
tokens, provider facts, who installed it and, for pasted tokens, the
installation they rotate, ids.Nil() for none.
*/
type installation struct {
	provider string
	account  string
	app      string
	name     string
	source   string
	scopes   []string
	creds    credentials
	metadata json.RawMessage
	member   ids.UUID
	rotate   ids.UUID
}

/*
create stores a new installation in the workspace.
*/
func (s *Services) create(ctx context.Context, q *models.Queries, workspaceID ids.UUID, in installation,
) (models.IntegrationInstallation, error) {
	id := ids.New()

	sealed, err := s.seal(ctx, workspaceID, installationAAD(id), in.creds)
	if err != nil {
		return models.IntegrationInstallation{}, err
	}

	row, err := q.CreateIntegrationInstallation(ctx, models.CreateIntegrationInstallationParams{
		ID: id, WorkspaceID: workspaceID, Provider: in.provider, ExternalAccountID: optional(in.account),
		ExternalAppID: optional(in.app), Name: cmp.Or(in.name, in.account), Source: in.source,
		Scopes: nonNil(in.scopes), CredentialsEncrypted: sealed, Metadata: in.metadata,
		InstalledByMemberID: &in.member, Now: s.Clock.Now(),
	})

	return row, saveError(err)
}

/*
reinstall stores fresh tokens on an existing installation.
*/
func (s *Services) reinstall(ctx context.Context, q *models.Queries, existing models.IntegrationInstallation,
	in installation,
) (models.IntegrationInstallation, error) {
	sealed, err := s.seal(ctx, existing.WorkspaceID, installationAAD(existing.ID), in.creds)
	if err != nil {
		return models.IntegrationInstallation{}, err
	}

	row, err := q.ReinstallIntegrationInstallation(ctx, models.ReinstallIntegrationInstallationParams{
		WorkspaceID: existing.WorkspaceID, ID: existing.ID, Name: cmp.Or(in.name, existing.Name),
		Scopes: nonNil(in.scopes), CredentialsEncrypted: sealed, Metadata: in.metadata, Source: in.source,
		InstalledByMemberID: &in.member, Now: s.Clock.Now(),
	})

	return row, saveError(err)
}

/*
saveError maps a write refused because another workspace holds the
account live.
*/
func saveError(err error) error {
	if database.IsUniqueViolation(err, takenIndex) {
		return exceptions.ErrIntegrationAccountTaken
	}

	if err != nil {
		return fmt.Errorf("save integration installation: %w", err)
	}

	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
