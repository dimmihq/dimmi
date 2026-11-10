/*
|--------------------------------------------------------------------------
| Start Installation
|--------------------------------------------------------------------------
|
| A teammate who manages integrations starts installing a tool's app
| from their own browser. The install asks for every scope the product's
| features need, so turning on another feature later needs no second
| consent, plus whatever the workspace's installation already holds.
| When the workspace's installation already holds every scope, there is
| nothing to approve: the features are added at once and the browser
| goes straight back.
|
*/

package integrations

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of attempt a callback finishes.
*/
const (
	kindInstall = "install"
	kindGrant   = "grant"
)

/*
StartInstallationInput is who installs, which provider, the features
the install is for, every feature when empty, the page to come back
to, and the browser session the attempt belongs to.
*/
type StartInstallationInput struct {
	Membership authz.Membership
	Provider   string
	Features   []string
	ReturnTo   string
	Binding    string
}

/*
StartInstallation begins an install and returns the URL to send the
browser to.

Errors: [exceptions.ErrForbidden], [exceptions.ErrIntegrationUnavailable],
[exceptions.ErrIntegrationFeatureUnknown],
[exceptions.ErrIntegrationReturnTo], or a wrapped error.
*/
type StartInstallation struct {
	*Services
}

/*
Handle checks the request and starts the attempt.
*/
func (a *StartInstallation) Handle(ctx context.Context, in StartInstallationInput) (string, error) {
	if err := requireManage(in.Membership); err != nil {
		return "", err
	}

	if !a.Installable(in.Provider) {
		return "", exceptions.ErrIntegrationUnavailable
	}

	for _, feature := range in.Features {
		if !a.Registry.HasFeature(in.Provider, feature) {
			return "", exceptions.ErrIntegrationFeatureUnknown
		}
	}

	returnTo, err := ReturnTo(in.ReturnTo)
	if err != nil {
		return "", err
	}

	features := a.Registry.FeatureNames(in.Provider, in.Features)
	scopes := a.Registry.BotScopes(in.Provider, nil)

	covering, err := a.covering(ctx, in, scopes)
	if err != nil {
		return "", err
	}

	if covering != nil {
		in.Features, in.ReturnTo = features, returnTo

		return a.addFeatures(ctx, in, *covering)
	}

	authURL, _, err := a.OAuth.Redirect(ctx, in.Provider, oauthclient.Request{
		Scopes: scopes, Binding: in.Binding,
		Payload: oauthclient.Payload{
			"kind": kindInstall, "workspace_id": in.Membership.Workspace.ID.String(),
			"member_id": in.Membership.Member.ID.String(), "return_to": returnTo,
			"features": strings.Join(features, ","),
		},
	})

	return authURL, err
}

/*
covering returns the workspace's active installation of the provider
that already holds every scope, nil when none does.
*/
func (a *StartInstallation) covering(ctx context.Context, in StartInstallationInput, scopes []string,
) (*models.IntegrationInstallation, error) {
	var found *models.IntegrationInstallation

	workspaceID := in.Membership.Workspace.ID

	err := a.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		rows, err := q.ListIntegrationInstallations(ctx, models.ListIntegrationInstallationsParams{
			WorkspaceID: in.Membership.Workspace.ID, Provider: &in.Provider,
		})

		i := slices.IndexFunc(rows, func(r models.IntegrationInstallation) bool {
			return r.Status == StatusActive && len(oauth.Missing(scopes, r.Scopes)) == 0
		})
		if i >= 0 {
			found = &rows[i]
		}

		return err
	})

	return found, err
}

/*
addFeatures announces the input's features on an installation that
already has their scopes and returns the page to come back to.
*/
func (a *StartInstallation) addFeatures(ctx context.Context, in StartInstallationInput,
	row models.IntegrationInstallation,
) (string, error) {
	err := a.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, _ *models.Queries, tx *database.Tx) error {
		return a.Events.Dispatch(ctx, tx, appevents.IntegrationConnected{
			Base: a.base(row.WorkspaceID, in.Membership.Member.ID), InstallationID: row.ID, Provider: row.Provider,
			ExternalAccountID: deref(row.ExternalAccountID), ExternalAppID: deref(row.ExternalAppID),
			Source: row.Source, Features: in.Features, Scopes: row.Scopes,
		})
	})
	if err != nil {
		return "", err
	}

	return Landing(in.ReturnTo, url.Values{"connected": {publicID(row.ID)}}), nil
}

func publicID(id ids.UUID) string {
	return models.PublicID(models.PrefixIntegrationInstallation, id)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
