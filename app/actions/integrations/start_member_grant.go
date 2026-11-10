/*
|--------------------------------------------------------------------------
| Start Member Grant
|--------------------------------------------------------------------------
|
| Any teammate may connect their own account on a working installation,
| so what they send through the tool goes out as them rather than as the
| app. The grant asks only for the user scopes the product's features
| registered, keeps the teammate on the installation's own account, and
| is bound to their browser session: it is theirs alone.
|
*/

package integrations

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
StartMemberGrantInput is the teammate connecting their account, the
installation, the page to come back to, and their browser session.
*/
type StartMemberGrantInput struct {
	Membership     authz.Membership
	InstallationID ids.UUID
	ReturnTo       string
	Binding        string
}

/*
StartMemberGrant begins a teammate's grant and returns the URL to send
their browser to.

Errors: [exceptions.ErrNotFound], [exceptions.ErrIntegrationNotActive],
[exceptions.ErrIntegrationNoUserScopes],
[exceptions.ErrIntegrationUnavailable],
[exceptions.ErrIntegrationReturnTo], or a wrapped error.
*/
type StartMemberGrant struct {
	*Services
}

/*
Handle checks the installation and starts the attempt.
*/
func (a *StartMemberGrant) Handle(ctx context.Context, in StartMemberGrantInput) (string, error) {
	returnTo, err := ReturnTo(in.ReturnTo)
	if err != nil {
		return "", err
	}

	var row models.IntegrationInstallation

	workspaceID := in.Membership.Workspace.ID

	err = a.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		row, err = findInstallation(ctx, q, workspaceID, in.InstallationID)

		return err
	})
	if err != nil {
		return "", err
	}

	scopes, extra, err := a.grantRequest(row)
	if err != nil {
		return "", err
	}

	authURL, _, err := a.OAuth.Redirect(ctx, row.Provider, oauthclient.Request{
		UserScopes: scopes, Extra: extra, Binding: in.Binding,
		Payload: oauthclient.Payload{
			"kind": kindGrant, "workspace_id": row.WorkspaceID.String(), "installation_id": row.ID.String(),
			"member_id": in.Membership.Member.ID.String(), "return_to": returnTo,
		},
	})

	return authURL, err
}

/*
grantRequest returns the user scopes and the provider parameters a
teammate's grant on the installation asks for.
*/
func (a *StartMemberGrant) grantRequest(row models.IntegrationInstallation) ([]string, map[string]string, error) {
	if row.Status != StatusActive {
		return nil, nil, exceptions.ErrIntegrationNotActive
	}

	p, err := a.provider(row.Provider)
	if err != nil || !a.OAuth.Has(row.Provider) {
		return nil, nil, exceptions.ErrIntegrationUnavailable
	}

	scopes := a.Registry.UserScopes(row.Provider)
	if len(scopes) == 0 {
		return nil, nil, exceptions.ErrIntegrationNoUserScopes
	}

	var extra map[string]string
	if p.Hint != nil && row.ExternalAccountID != nil {
		extra = p.Hint(*row.ExternalAccountID)
	}

	return scopes, extra, nil
}
