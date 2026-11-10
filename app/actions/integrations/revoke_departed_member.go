/*
|--------------------------------------------------------------------------
| Revoke Departed Member
|--------------------------------------------------------------------------
|
| A grant never outlives its teammate: when a member loses access to the
| workspace, every grant they hold on its installations is deleted, the
| links it proved go with it, and each token is given back to the
| provider, best effort, once the change is stored.
|
*/

package integrations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokeDepartedMember removes a departed member's grants.

Errors: a wrapped error.
*/
type RevokeDepartedMember struct {
	*Services
}

/*
departed is a grant to give back once it is gone.
*/
type departed struct {
	provider string
	token    string
}

/*
Handle removes the member's grants in the workspace.
*/
func (a *RevokeDepartedMember) Handle(ctx context.Context, workspaceID, memberID ids.UUID) error {
	var gone []departed

	err := a.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		grants, err := q.ListMemberIntegrationGrants(ctx, models.ListMemberIntegrationGrantsParams{
			WorkspaceID: workspaceID, MemberID: memberID,
		})
		if err != nil {
			return fmt.Errorf("list member integration grants: %w", err)
		}

		for _, grant := range grants {
			d, err := a.remove(ctx, q, grant)
			if err != nil {
				return err
			}

			gone = append(gone, d)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, d := range gone {
		a.revokeQuietly(ctx, d.provider, d.token)
	}

	return nil
}

/*
remove deletes one grant with its links and audits it, returning its
token when its installation is still live.
*/
func (a *RevokeDepartedMember) remove(ctx context.Context, q *models.Queries, grant models.IntegrationMemberGrant,
) (departed, error) {
	row, err := findInstallation(ctx, q, grant.WorkspaceID, grant.IntegrationInstallationID)
	if err != nil {
		return departed{}, wrap("delete integration member grant",
			q.DeleteIntegrationMemberGrant(ctx, models.DeleteIntegrationMemberGrantParams{
				WorkspaceID: grant.WorkspaceID, ID: grant.ID,
			}))
	}

	var d departed
	if creds, err := a.open(ctx, grant.CredentialsEncrypted, grantAAD(row.ID, grant.MemberID)); err == nil {
		d = departed{provider: row.Provider, token: creds.Access}
	}

	if err := q.UnlinkIntegrationMember(ctx, models.UnlinkIntegrationMemberParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: &row.ID, MemberID: grant.MemberID,
	}); err != nil {
		return d, fmt.Errorf("unlink integration member: %w", err)
	}

	if err := q.DeleteIntegrationMemberGrant(ctx, models.DeleteIntegrationMemberGrantParams{
		WorkspaceID: grant.WorkspaceID, ID: grant.ID,
	}); err != nil {
		return d, fmt.Errorf("delete integration member grant: %w", err)
	}

	return d, a.record(ctx, row, note{actor: ids.Nil(), event: AuditGrantRevoked, changes: map[string]any{
		"member_id": grant.MemberID.String(), "external_user_id": grant.ExternalUserID, "reason": "member_departed",
	}})
}
