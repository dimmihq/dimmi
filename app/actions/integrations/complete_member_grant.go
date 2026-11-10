/*
|--------------------------------------------------------------------------
| Complete Member Grant
|--------------------------------------------------------------------------
|
| A teammate's grant comes back from the provider with a token acting as
| them and their id there. It must be for the installation's own
| account. The provider has just proven who they are, so the grant also
| links them to that provider user on every channel the installation
| backs, replacing an older link of theirs, and replacing anyone else's
| claim on that provider user, whose proof is older.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CompleteMemberGrant stores teammates' grants. The callback runs it.
*/
type CompleteMemberGrant struct {
	*Services
}

/*
complete stores the grant for the teammate and returns the installation
it is on.
*/
func (a *CompleteMemberGrant) complete(ctx context.Context, m authz.Membership, grant oauthclient.Grant,
	payload oauthclient.Payload,
) (ids.UUID, error) {
	installationID, err := ids.Parse(payload["installation_id"])
	if err != nil {
		return ids.Nil(), exceptions.ErrIntegrationState
	}

	var row models.IntegrationInstallation

	err = a.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		row, err = findInstallation(ctx, q, m.Workspace.ID, installationID)
		if err != nil {
			return err
		}

		if err := checkGrant(row, grant); err != nil {
			return err
		}

		return a.store(ctx, q, row, proof{member: m.Member.ID, grant: grant})
	})
	if errors.Is(err, exceptions.ErrIntegrationAccountMismatch) {
		a.revokeQuietly(ctx, row.Provider, value(grant.UserAccess))
	}

	return row.ID, err
}

/*
checkGrant refuses a grant on an installation that does not work, one
without a token acting as the teammate, and one for another account.
*/
func checkGrant(row models.IntegrationInstallation, grant oauthclient.Grant) error {
	if row.Status != StatusActive {
		return exceptions.ErrIntegrationNotActive
	}

	if grant.UserAccess == nil || grant.UserID == "" {
		return exceptions.ErrIntegrationProvider
	}

	if grant.Account != "" && grant.Account != deref(row.ExternalAccountID) {
		return exceptions.ErrIntegrationAccountMismatch
	}

	return nil
}

/*
proof is a teammate's grant: who they are here, and what the provider
returned for them.
*/
type proof struct {
	member ids.UUID
	grant  oauthclient.Grant
}

/*
store saves the grant, links the teammate on the installation's
channels and audits it.
*/
func (a *CompleteMemberGrant) store(ctx context.Context, q *models.Queries, row models.IntegrationInstallation,
	p proof,
) error {
	member, grant := p.member, p.grant

	previous, err := q.FindIntegrationGrantOfMember(ctx, models.FindIntegrationGrantOfMemberParams{
		WorkspaceID: row.WorkspaceID, IntegrationInstallationID: row.ID, MemberID: member,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find integration member grant: %w", err)
	}

	replaced := err == nil && previous.Status == StatusActive && previous.ExternalUserID != grant.UserID

	if err := a.save(ctx, q, row, p); err != nil {
		return err
	}

	event := AuditGrantCreated
	changes := map[string]any{"member_id": member.String(), "external_user_id": grant.UserID}

	if replaced {
		event = AuditGrantReplaced
		changes["external_user_id"] = map[string]any{"from": previous.ExternalUserID, "to": grant.UserID}
	}

	return a.record(ctx, row, note{actor: member, event: event, changes: changes})
}

/*
save writes the grant and the teammate's links.
*/
func (a *CompleteMemberGrant) save(ctx context.Context, q *models.Queries, row models.IntegrationInstallation,
	p proof,
) error {
	member, grant := p.member, p.grant

	sealed, err := a.seal(ctx, row.WorkspaceID, grantAAD(row.ID, member), userCredentials(grant))
	if err != nil {
		return err
	}

	ws, user, now := row.WorkspaceID, grant.UserID, a.Clock.Now()

	if _, err := q.DeleteIntegrationUserGrant(ctx, models.DeleteIntegrationUserGrantParams{
		WorkspaceID: ws, IntegrationInstallationID: row.ID, ExternalUserID: user, MemberID: member,
	}); err != nil {
		return fmt.Errorf("replace integration user grant: %w", err)
	}

	if _, err := q.SaveIntegrationMemberGrant(ctx, models.SaveIntegrationMemberGrantParams{
		WorkspaceID: ws, IntegrationInstallationID: row.ID, MemberID: member, ExternalUserID: user,
		Scopes: nonNil(grant.UserScopes), CredentialsEncrypted: sealed, Now: now,
	}); err != nil {
		return fmt.Errorf("save integration member grant: %w", err)
	}

	identity := channelIdentity(deref(row.ExternalAccountID), user)
	if err := q.ClearIntegrationMemberLinks(ctx, models.ClearIntegrationMemberLinksParams{
		WorkspaceID: ws, IntegrationInstallationID: &row.ID, MemberID: member, ExternalUserID: identity,
	}); err != nil {
		return fmt.Errorf("clear integration member links: %w", err)
	}

	return wrap("link integration member", q.LinkIntegrationMember(ctx, models.LinkIntegrationMemberParams{
		WorkspaceID: ws, IntegrationInstallationID: &row.ID, MemberID: member, ExternalUserID: identity, Now: now,
	}))
}

/*
channelIdentity is how chat channels know a provider user: by the
account they belong to and their id in it, since the same id can name
different people in different accounts.
*/
func channelIdentity(account, user string) string {
	return account + ":" + user
}

/*
wrap says what was being attempted when the error happened.
*/
func wrap(attempt string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", attempt, err)
	}

	return nil
}
