/*
|--------------------------------------------------------------------------
| Revoke Member Grant
|--------------------------------------------------------------------------
|
| A teammate may take back their own grant at any time, and whoever
| manages integrations may take back anyone's. The token is wiped at
| once, the teammate's provider-proven links on the installation's
| channels go with it, and the token is given back to the provider,
| best effort, once the change is stored.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RevokeMemberGrantInput is who revokes, the installation and the grant.
*/
type RevokeMemberGrantInput struct {
	Membership     authz.Membership
	InstallationID ids.UUID
	GrantID        ids.UUID
}

/*
RevokeMemberGrant takes a teammate's grant back.

Errors: [exceptions.ErrNotFound], [exceptions.ErrForbidden], or a
wrapped error.
*/
type RevokeMemberGrant struct {
	*Services
}

/*
Handle revokes the grant.
*/
func (a *RevokeMemberGrant) Handle(ctx context.Context, in RevokeMemberGrantInput) error {
	m := in.Membership

	var (
		row    models.IntegrationInstallation
		tokens []string
	)

	err := a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		var err error
		if row, err = findInstallation(ctx, q, m.Workspace.ID, in.InstallationID); err != nil {
			return err
		}

		grant, err := q.FindIntegrationMemberGrant(ctx, models.FindIntegrationMemberGrantParams{
			WorkspaceID: m.Workspace.ID, IntegrationInstallationID: row.ID, ID: in.GrantID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find integration member grant: %w", err)
		}

		if grant.MemberID != m.Member.ID && !m.Can(ManagePermission) {
			return exceptions.ErrForbidden
		}

		tokens, err = a.revokeGrants(ctx, q, grantSet{row: row, grants: []models.IntegrationMemberGrant{grant}},
			m.Member.ID)

		return err
	})
	if err != nil {
		return err
	}

	for _, token := range tokens {
		a.revokeQuietly(ctx, row.Provider, token)
	}

	return nil
}

/*
grantSet is grants on one installation.
*/
type grantSet struct {
	row    models.IntegrationInstallation
	grants []models.IntegrationMemberGrant
}

/*
revokeGrants marks the grants revoked, wipes their tokens, unlinks
their teammates and audits each, returning the tokens they held so the
caller can give them back once the change is stored.
*/
func (s *Services) revokeGrants(ctx context.Context, q *models.Queries, set grantSet, actorID ids.UUID,
) ([]string, error) {
	row, grants := set.row, set.grants

	var tokens []string

	for _, grant := range grants {
		if creds, err := s.open(ctx, grant.CredentialsEncrypted, grantAAD(row.ID, grant.MemberID)); err == nil {
			tokens = append(tokens, creds.Access)
		}

		if err := s.revokeGrant(ctx, q, row, grant); err != nil {
			return nil, err
		}

		err := s.record(ctx, row, note{actor: actorID, event: AuditGrantRevoked, changes: map[string]any{
			"member_id": grant.MemberID.String(), "external_user_id": grant.ExternalUserID,
		}})
		if err != nil {
			return nil, err
		}
	}

	return tokens, nil
}

/*
revokeGrant marks one grant revoked with a sealed empty token and
removes its teammate's links.
*/
func (s *Services) revokeGrant(ctx context.Context, q *models.Queries, row models.IntegrationInstallation,
	grant models.IntegrationMemberGrant,
) error {
	sealed, err := s.seal(ctx, row.WorkspaceID, grantAAD(row.ID, grant.MemberID), credentials{})
	if err != nil {
		return err
	}

	if _, err := q.RevokeIntegrationMemberGrant(ctx, models.RevokeIntegrationMemberGrantParams{
		WorkspaceID: row.WorkspaceID, ID: grant.ID, CredentialsEncrypted: sealed, Now: s.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("revoke integration member grant: %w", err)
	}

	return wrap("unlink integration member", q.UnlinkIntegrationMember(ctx, models.UnlinkIntegrationMemberParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: &row.ID, MemberID: grant.MemberID,
	}))
}

/*
activeGrants returns the installation's working grants, of the provider
users named when any are.
*/
func activeGrants(ctx context.Context, q *models.Queries, row models.IntegrationInstallation, users []string,
) ([]models.IntegrationMemberGrant, error) {
	grants, err := q.ListIntegrationMemberGrants(ctx, models.ListIntegrationMemberGrantsParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: row.ID, ExternalUserIds: nonNil(users),
	})
	if err != nil {
		return nil, fmt.Errorf("list integration member grants: %w", err)
	}

	return grants, nil
}
