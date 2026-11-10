/*
|--------------------------------------------------------------------------
| Revoke A Workspace's Integrations
|--------------------------------------------------------------------------
|
| A workspace deleted for good gives every token it holds back to the
| provider first, the installations' and every teammate's, so the app
| leaves the customer's Slack instead of staying installed and being
| sent events nobody reads. The provider is asked best effort: a token
| it no longer knows, or a provider that is down, never stops the purge,
| since the rows and their sealed tokens are deleted right after.
|
*/

package integrations

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokeWorkspace gives back every token of a workspace being deleted.
*/
type RevokeWorkspace struct {
	*Services
}

/*
handover is one token to give back and the provider it came from.
*/
type handover struct {
	provider string
	token    string
}

/*
Handle revokes the workspace's installation and teammate tokens at
their providers.
*/
func (a *RevokeWorkspace) Handle(ctx context.Context, workspaceID ids.UUID) error {
	var tokens []handover

	err := a.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		rows, err := q.ListIntegrationInstallations(ctx, models.ListIntegrationInstallationsParams{
			WorkspaceID: workspaceID,
		})
		if err != nil {
			return wrap("list integration installations", err)
		}

		for _, row := range rows {
			found, err := a.tokensOf(ctx, q, row)
			if err != nil {
				return err
			}

			tokens = append(tokens, found...)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, t := range tokens {
		a.revokeQuietly(ctx, t.provider, t.token)
	}

	return nil
}

/*
tokensOf opens the installation's token and its teammates' grants,
skipping any that no longer open.
*/
func (a *RevokeWorkspace) tokensOf(ctx context.Context, q *models.Queries, row models.IntegrationInstallation,
) ([]handover, error) {
	var out []handover

	if creds, err := a.open(ctx, row.CredentialsEncrypted, installationAAD(row.ID)); err == nil {
		out = append(out, handover{provider: row.Provider, token: creds.Access})
	}

	grants, err := activeGrants(ctx, q, row, nil)
	if err != nil {
		return nil, err
	}

	for _, grant := range grants {
		if creds, err := a.open(ctx, grant.CredentialsEncrypted, grantAAD(row.ID, grant.MemberID)); err == nil {
			out = append(out, handover{provider: row.Provider, token: creds.Access})
		}
	}

	return out, nil
}
