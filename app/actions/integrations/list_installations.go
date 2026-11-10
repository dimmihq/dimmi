/*
|--------------------------------------------------------------------------
| List Installations
|--------------------------------------------------------------------------
|
| Every teammate sees which tools this install offers, and for each
| installation its health, the scopes it holds, which features it is
| missing scopes for, what uses it, how many teammates connected their
| own account and whether they did: any teammate may connect theirs, so
| any teammate must find the installation. Tokens never leave.
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
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ProviderView is a provider this install offers. Installable says its
shared app can install over OAuth; without it, only pasted tokens of a
custom app connect it.
*/
type ProviderView struct {
	Provider    oauth.Provider
	Installable bool
	Features    []oauth.Feature
}

/*
FeatureState is one feature on an installation and the scopes it still
needs.
*/
type FeatureState struct {
	Name    string
	Missing []string
}

/*
InstallationView is an installation with its features, how many
teammates hold a grant on it, the channel connections it backs and the
viewer's own grant, nil when they have none.
*/
type InstallationView struct {
	Installation models.IntegrationInstallation
	Features     []FeatureState
	Grants       int64
	Consumers    []ids.UUID
	Own          *models.IntegrationMemberGrant
}

/*
ListInstallations lists providers and installations.

Errors: [exceptions.ErrNotFound], or a wrapped error.
*/
type ListInstallations struct {
	*Services
}

/*
Providers returns the providers this install offers.
*/
func (a *ListInstallations) Providers() []ProviderView {
	var out []ProviderView

	for _, p := range a.Registry.Providers() {
		if a.Enabled(p.Name) {
			out = append(out, ProviderView{
				Provider: p, Installable: a.Installable(p.Name), Features: a.Registry.Features(p.Name),
			})
		}
	}

	return out
}

/*
List returns the workspace's installations, of one provider when one is
named.
*/
func (a *ListInstallations) List(ctx context.Context, m authz.Membership, provider string,
) ([]InstallationView, error) {
	var out []InstallationView

	err := a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		rows, err := q.ListIntegrationInstallations(ctx, models.ListIntegrationInstallationsParams{
			WorkspaceID: m.Workspace.ID, Provider: optional(provider),
		})
		if err != nil {
			return fmt.Errorf("list integration installations: %w", err)
		}

		for _, row := range rows {
			view, err := a.view(ctx, q, m, row)
			if err != nil {
				return err
			}

			out = append(out, view)
		}

		return nil
	})

	return out, err
}

/*
Show returns one installation.
*/
func (a *ListInstallations) Show(ctx context.Context, m authz.Membership, id ids.UUID) (InstallationView, error) {
	var out InstallationView

	err := a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		row, err := findInstallation(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		out, err = a.view(ctx, q, m, row)

		return err
	})

	return out, err
}

/*
view gathers what is shown about an installation.
*/
func (a *ListInstallations) view(ctx context.Context, q *models.Queries, m authz.Membership,
	row models.IntegrationInstallation,
) (InstallationView, error) {
	view := InstallationView{Installation: row}

	for _, f := range a.Registry.Features(row.Provider) {
		missing := oauth.Missing(f.Scopes.Bot, row.Scopes)
		view.Features = append(view.Features, FeatureState{Name: f.Name, Missing: missing})
	}

	count, err := q.CountIntegrationMemberGrants(ctx, models.CountIntegrationMemberGrantsParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: row.ID,
	})
	if err != nil {
		return view, fmt.Errorf("count integration member grants: %w", err)
	}

	consumers, err := q.ListIntegrationConsumers(ctx, models.ListIntegrationConsumersParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: &row.ID,
	})
	if err != nil {
		return view, fmt.Errorf("list integration consumers: %w", err)
	}

	own, err := q.FindIntegrationGrantOfMember(ctx, models.FindIntegrationGrantOfMemberParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: row.ID, MemberID: m.Member.ID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return view, fmt.Errorf("find integration member grant: %w", err)
	}

	if err == nil {
		view.Own = &own
	}

	view.Grants, view.Consumers = count, consumers

	return view, nil
}
