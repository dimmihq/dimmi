/*
|--------------------------------------------------------------------------
| Create Manual Installation
|--------------------------------------------------------------------------
|
| A self-hosted install without a shared app connects a tool with tokens
| pasted from a custom app of its own. The token is checked with the
| provider first, which also says which account and app it belongs to,
| and the result is the same installation an OAuth install makes: the
| features that use it cannot tell the difference. Pasting new tokens on
| an existing installation rotates them, and only works for the same
| account.
|
*/

package integrations

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ManualInstallationInput is who connects, the provider, the installation
to rotate, ids.Nil() for a new one, the app token with the signing
secret and app-level token a custom app may have, and the features the
installation is for, every feature when empty.
*/
type ManualInstallationInput struct {
	Membership     authz.Membership
	Provider       string
	InstallationID ids.UUID
	Token          string
	SigningSecret  string
	AppToken       string
	Features       []string
}

/*
CreateManualInstallation connects a tool from pasted tokens. Teammates
who manage integrations or channels may.

Errors: [exceptions.ErrForbidden], [exceptions.ErrIntegrationUnavailable],
[exceptions.ErrIntegrationAuthFailed], [exceptions.ErrIntegrationProvider],
[exceptions.ErrIntegrationAccountMismatch],
[exceptions.ErrIntegrationAccountTaken], or a wrapped error.
*/
type CreateManualInstallation struct {
	*Services
}

/*
Handle checks the token and stores the installation.
*/
func (a *CreateManualInstallation) Handle(ctx context.Context, in ManualInstallationInput,
) (models.IntegrationInstallation, error) {
	m := in.Membership
	if !m.Can(ManagePermission) && !m.Can("channels:manage") {
		return models.IntegrationInstallation{}, exceptions.ErrForbidden
	}

	p, err := a.provider(in.Provider)
	if err != nil || p.Check == nil {
		return models.IntegrationInstallation{}, exceptions.ErrIntegrationUnavailable
	}

	health, err := p.Check(ctx, in.Token)
	if errors.Is(err, oauth.ErrAuthFailed) {
		return models.IntegrationInstallation{}, exceptions.ErrIntegrationAuthFailed
	}

	if err != nil {
		return models.IntegrationInstallation{}, exceptions.ErrIntegrationProvider
	}

	install := installation{
		provider: p.Name, account: health.Account, app: health.App, name: health.AccountName, source: SourceManual,
		scopes: health.Scopes, metadata: encodeMetadata(health.Metadata), member: m.Member.ID,
		creds:  credentials{Access: in.Token, SigningSecret: in.SigningSecret, AppToken: in.AppToken},
		rotate: in.InstallationID,
	}

	var row models.IntegrationInstallation

	err = a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, tx *database.Tx) error {
		saved, earlier, err := a.store(ctx, q, m.Workspace.ID, install)
		if err != nil {
			return err
		}

		row = saved
		features := a.Registry.FeatureNames(p.Name, in.Features)

		return a.announce(ctx, tx, m.Member.ID, announcement{row: saved, earlier: earlier, features: features})
	})

	return row, err
}

/*
store rotates the named installation's tokens, refusing another account,
or saves the workspace's installation of the account.
*/
func (a *CreateManualInstallation) store(ctx context.Context, q *models.Queries, workspaceID ids.UUID,
	in installation,
) (models.IntegrationInstallation, *models.IntegrationInstallation, error) {
	id := in.rotate

	var (
		existing models.IntegrationInstallation
		err      error
	)

	if id != ids.Nil() {
		existing, err = findInstallation(ctx, q, workspaceID, id)
		if err == nil && (deref(existing.ExternalAccountID) != in.account || existing.Provider != in.provider) {
			err = exceptions.ErrIntegrationAccountMismatch
		}
	} else {
		existing, err = q.FindIntegrationInstallationByAccount(ctx, models.FindIntegrationInstallationByAccountParams{
			WorkspaceID: workspaceID, Provider: in.provider, ExternalAppID: optional(in.app),
			ExternalAccountID: optional(in.account),
		})
	}

	if errors.Is(err, database.ErrNotFound) {
		row, err := a.create(ctx, q, workspaceID, in)

		return row, nil, err
	}

	if err != nil {
		return models.IntegrationInstallation{}, nil, err
	}

	row, err := a.reinstall(ctx, q, existing, in)

	return row, &existing, err
}
