/*
|--------------------------------------------------------------------------
| Complete Installation
|--------------------------------------------------------------------------
|
| The provider sends the browser back with a code. The attempt counts
| only in the browser session that started it, for a teammate who still
| manages integrations in that workspace. One provider account feeds one
| workspace per app: installing the account again in the same workspace
| refreshes the installation it already has, reconnecting it when it
| was disconnected, and another workspace's live installation of it
| refuses the install. An Enterprise Grid organization install is
| refused and its token given back.
|
| Every feature that uses the provider hears of the installation through
| integration.connected, with the features the install was for.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/integrations/oauth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
takenIndex is the index that keeps one live installation per provider
account and app.
*/
const takenIndex = "integration_installations_provider_account_key"

/*
CallbackInput is the provider's callback: which provider, the browser
session it arrived in and the query it carried.
*/
type CallbackInput struct {
	Provider string
	Binding  string
	Query    url.Values
}

/*
CallbackResult is where the browser goes next: the page the attempt
came from, with the installation it connected or the code of what went
wrong.
*/
type CallbackResult struct {
	ReturnTo       string
	InstallationID ids.UUID
	Error          string
}

/*
Location returns the page to send the browser to.
*/
func (r CallbackResult) Location() string {
	if r.Error != "" {
		return Landing(r.ReturnTo, url.Values{"error": {r.Error}})
	}

	return Landing(r.ReturnTo, url.Values{"connected": {publicID(r.InstallationID)}})
}

/*
CompleteCallback finishes installs and member grants.

Errors: [exceptions.ErrIntegrationState] for a callback that is unknown,
expired, used, or opened in another browser or by another teammate;
failures the person can act on come back as the result's error code.
*/
type CompleteCallback struct {
	*Services
	Resolve *workspaces.ResolveWorkspace
	Grants  *CompleteMemberGrant
}

/*
Handle finishes the attempt the callback belongs to.
*/
func (a *CompleteCallback) Handle(ctx context.Context, in CallbackInput) (CallbackResult, error) {
	grant, payload, err := a.OAuth.Callback(ctx, in.Provider, in.Binding, in.Query)
	if err != nil && payload == nil {
		return CallbackResult{}, exceptions.ErrIntegrationState
	}

	result := CallbackResult{ReturnTo: payload["return_to"], Error: providerFailure(err)}
	if result.Error != "" {
		return result, nil
	}

	m, err := a.member(ctx, payload)
	if err != nil {
		return CallbackResult{}, err
	}

	var id ids.UUID
	if payload["kind"] == kindGrant {
		id, err = a.Grants.complete(ctx, m, grant, payload)
	} else {
		id, err = a.install(ctx, m, attempt{provider: in.Provider, grant: grant, payload: payload})
	}

	return finish(result, id, err)
}

/*
providerFailure returns the code of a callback the provider refused, or
empty when it did not.
*/
func providerFailure(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, oauthclient.ErrDenied):
		return exceptions.ErrIntegrationDenied.Code()
	default:
		return exceptions.ErrIntegrationProvider.Code()
	}
}

/*
attempt is a finished attempt: its provider, the grant it returned and
what it was started with.
*/
type attempt struct {
	provider string
	grant    oauthclient.Grant
	payload  oauthclient.Payload
}

/*
finish records the outcome: a failure the person can act on as its
code, anything else as the error.
*/
func finish(result CallbackResult, id ids.UUID, err error) (CallbackResult, error) {
	if code := fault.CodeOf(err); code != "" {
		result.Error = code

		return result, nil
	}

	if err != nil {
		return CallbackResult{}, err
	}

	result.InstallationID = id

	return result, nil
}

/*
member returns the caller's membership of the workspace the attempt was
started in, when they are still the teammate who started it.
*/
func (a *CompleteCallback) member(ctx context.Context, payload oauthclient.Payload) (authz.Membership, error) {
	principal, ok := appauth.CurrentPrincipal(ctx)
	workspaceID, err := ids.Parse(payload["workspace_id"])
	if !ok || err != nil {
		return authz.Membership{}, exceptions.ErrIntegrationState
	}

	m, err := a.Resolve.Handle(ctx, workspaces.ResolveWorkspaceInput{
		Principal: principal, Named: models.PublicID(models.PrefixWorkspace, workspaceID), Writes: true,
	})
	if err != nil {
		return authz.Membership{}, err
	}

	if m.Member.ID.String() != payload["member_id"] {
		return authz.Membership{}, exceptions.ErrIntegrationState
	}

	return m, nil
}

/*
install stores the installation the grant is for and announces it.
*/
func (a *CompleteCallback) install(ctx context.Context, m authz.Membership, at attempt) (ids.UUID, error) {
	if err := requireManage(m); err != nil {
		return ids.Nil(), err
	}

	p, err := a.provider(at.provider)
	if err != nil {
		return ids.Nil(), err
	}

	if err := validate(p, at.grant); err != nil {
		a.revokeQuietly(ctx, at.provider, value(at.grant.Access))

		return ids.Nil(), err
	}

	install := granted(p, at.grant, m.Member.ID)
	features := splitList(at.payload["features"])

	var row models.IntegrationInstallation

	err = a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, tx *database.Tx) error {
		saved, earlier, err := a.save(ctx, q, m.Workspace.ID, install)
		if err != nil {
			return err
		}

		row = saved

		return a.announce(ctx, tx, m.Member.ID, announcement{row: saved, earlier: earlier, features: features})
	})

	return row.ID, err
}

/*
validate refuses a grant without an app token or account, and one the
provider's own rules refuse.
*/
func validate(p oauth.Provider, grant oauthclient.Grant) error {
	if grant.Access == nil || grant.Account == "" {
		return exceptions.ErrIntegrationProvider
	}

	if p.Validate == nil {
		return nil
	}

	return p.Validate(grant)
}

/*
save stores the grant on the workspace's installation of the account,
or on a new one, and returns it with the installation as it was before,
nil when it is new.
*/
func (a *CompleteCallback) save(ctx context.Context, q *models.Queries, workspaceID ids.UUID, install installation,
) (models.IntegrationInstallation, *models.IntegrationInstallation, error) {
	existing, err := q.FindIntegrationInstallationByAccount(ctx, models.FindIntegrationInstallationByAccountParams{
		WorkspaceID: workspaceID, Provider: install.provider, ExternalAppID: &install.app,
		ExternalAccountID: &install.account,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return models.IntegrationInstallation{}, nil, fmt.Errorf("find integration installation: %w", err)
	}

	if err != nil {
		row, err := a.create(ctx, q, workspaceID, install)

		return row, nil, err
	}

	row, err := a.reinstall(ctx, q, existing, install)

	return row, &existing, err
}

/*
granted is the installation an OAuth grant makes, installed by the
member, with the facts the provider keeps from it.
*/
func granted(p oauth.Provider, grant oauthclient.Grant, member ids.UUID) installation {
	var facts map[string]any
	if p.Metadata != nil {
		facts = p.Metadata(grant)
	}

	return installation{
		provider: p.Name, account: grant.Account, app: grant.App, name: grant.AccountName, source: SourceOAuth,
		scopes: grant.Scopes, creds: botCredentials(grant), metadata: encodeMetadata(facts), member: member,
	}
}

/*
splitList reads a comma-joined list.
*/
func splitList(list string) []string {
	if list == "" {
		return nil
	}

	return strings.Split(list, ",")
}

/*
changed returns the scopes added and removed between two grants.
*/
func changed(before, after []string) ([]string, []string) {
	return oauth.Missing(after, before), oauth.Missing(before, after)
}

/*
sameScopes reports whether two grants hold the same scopes.
*/
func sameScopes(before, after []string) bool {
	added, removed := changed(before, after)

	return len(added) == 0 && len(removed) == 0
}

/*
announcement is what an install tells the features: the installation,
how it was before, nil when new, and the features the install was for.
*/
type announcement struct {
	row      models.IntegrationInstallation
	earlier  *models.IntegrationInstallation
	features []string
}

/*
announce dispatches integration.connected, and integration.scopes_changed
when an existing installation's scopes changed, and audits both.
*/
func (s *Services) announce(ctx context.Context, tx *database.Tx, actorID ids.UUID, n announcement) error {
	row := n.row
	err := s.Events.Dispatch(ctx, tx, appevents.IntegrationConnected{
		Base: s.base(row.WorkspaceID, actorID), InstallationID: row.ID, Provider: row.Provider,
		ExternalAccountID: deref(row.ExternalAccountID), ExternalAppID: deref(row.ExternalAppID),
		Source: row.Source, Features: n.features, Scopes: row.Scopes, Reconnected: n.earlier != nil,
	})
	if err != nil {
		return fmt.Errorf("dispatch integration connected: %w", err)
	}

	err = s.record(ctx, row, note{actor: actorID, event: AuditConnected, changes: map[string]any{
		"provider": row.Provider, "account": deref(row.ExternalAccountID), "scopes": row.Scopes,
		"features": n.features, "source": row.Source,
	}})
	if err != nil || n.earlier == nil || sameScopes(n.earlier.Scopes, row.Scopes) {
		return err
	}

	added, removed := changed(n.earlier.Scopes, row.Scopes)
	if err := s.Events.Dispatch(ctx, tx, appevents.IntegrationScopesChanged{
		Base: s.base(row.WorkspaceID, actorID), InstallationID: row.ID, Provider: row.Provider,
		Added: added, Removed: removed,
	}); err != nil {
		return fmt.Errorf("dispatch integration scopes changed: %w", err)
	}

	return s.record(ctx, row, note{actor: actorID, event: AuditScopesChanged, changes: map[string]any{
		"scopes": map[string]any{"from": n.earlier.Scopes, "to": row.Scopes},
	}})
}
