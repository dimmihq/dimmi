/*
|--------------------------------------------------------------------------
| Token Service
|--------------------------------------------------------------------------
|
| The only way a feature gets a token: it names the installation, and a
| member when it acts as that teammate, and never keeps a copy. A token
| close to expiry is renewed on the way out, by one process at a time:
| the others wait for the lock, a few seconds at most, and read the
| renewed token back. A provider refusing the renewal marks the
| installation broken, once, and every caller learns it the same way.
|
| The context must carry the workspace the installation belongs to.
|
*/

package integrations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
refreshWait is how long a caller waits for another process renewing the
same token.
*/
const refreshWait = 5 * time.Second

/*
CodeRefreshInvalidGrant is the error code of an installation whose
provider refused to renew its token.
*/
const CodeRefreshInvalidGrant = "refresh_invalid_grant"

/*
Tokens hands out installation and member tokens.
*/
type Tokens struct {
	*Services
}

/*
Bot returns the token acting as the app on the installation.

Errors: [ErrIntegrationDisconnected], [ErrIntegrationBroken], or a
wrapped error.
*/
func (t *Tokens) Bot(ctx context.Context, installationID ids.UUID) (string, error) {
	row, creds, err := t.installation(ctx, installationID)
	if err != nil {
		return "", err
	}

	if !creds.expiring(t.Clock.Now(), t.Config.RefreshSkew) {
		return creds.Access, nil
	}

	return t.renewInstallation(ctx, row)
}

/*
User returns the token acting as the member on the installation, and
who the member is at the provider.

Errors: [ErrIntegrationDisconnected], [ErrIntegrationBroken],
[ErrNoMemberGrant], or a wrapped error.
*/
func (t *Tokens) User(ctx context.Context, installationID, memberID ids.UUID) (string, string, error) {
	row, _, err := t.installation(ctx, installationID)
	if err != nil {
		return "", "", err
	}

	grant, creds, err := t.grant(ctx, row, memberID)
	if err != nil {
		return "", "", err
	}

	if !creds.expiring(t.Clock.Now(), t.Config.RefreshSkew) {
		return creds.Access, grant.ExternalUserID, nil
	}

	token, err := t.renewGrant(ctx, row, memberID)

	return token, grant.ExternalUserID, err
}

/*
installation reads a working installation of the context's workspace
with its tokens.
*/
func (t *Tokens) installation(ctx context.Context, id ids.UUID) (models.IntegrationInstallation, credentials, error) {
	tenant, err := tenancy.Required(ctx)
	if err != nil {
		return models.IntegrationInstallation{}, credentials{}, fmt.Errorf("integration token: %w", err)
	}

	workspaceID, err := ids.Parse(tenant)
	if err != nil {
		return models.IntegrationInstallation{}, credentials{}, fmt.Errorf("integration token: %w", err)
	}

	var row models.IntegrationInstallation

	err = t.inWorkspace(ctx, workspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		row, err = findInstallation(ctx, q, workspaceID, id)

		return err
	})

	switch {
	case errors.Is(err, exceptions.ErrNotFound) || row.Status == StatusDisconnected:
		return row, credentials{}, ErrIntegrationDisconnected
	case err != nil:
		return row, credentials{}, err
	case row.Status == StatusError:
		return row, credentials{}, ErrIntegrationBroken
	}

	creds, err := t.open(ctx, row.CredentialsEncrypted, installationAAD(row.ID))

	return row, creds, err
}

/*
grant reads the member's working grant on the installation with its
token.
*/
func (t *Tokens) grant(ctx context.Context, row models.IntegrationInstallation, memberID ids.UUID,
) (models.IntegrationMemberGrant, credentials, error) {
	var grant models.IntegrationMemberGrant

	err := t.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		var err error
		grant, err = q.FindIntegrationGrantOfMember(ctx, models.FindIntegrationGrantOfMemberParams{
			WorkspaceID: row.WorkspaceID, IntegrationInstallationID: row.ID, MemberID: memberID,
		})

		return err
	})

	switch {
	case errors.Is(err, database.ErrNotFound) || (err == nil && grant.Status != StatusActive):
		return grant, credentials{}, ErrNoMemberGrant
	case err != nil:
		return grant, credentials{}, fmt.Errorf("find integration member grant: %w", err)
	}

	creds, err := t.open(ctx, grant.CredentialsEncrypted, grantAAD(row.ID, memberID))

	return grant, creds, err
}

/*
locked runs the renewal under the lock of the name. A caller that cannot
take the lock in time reads the token back instead, renewed by whoever
held it.
*/
func (t *Tokens) locked(ctx context.Context, name string, reread, renew func(ctx context.Context) (string, error),
) (string, error) {
	wait, cancel := context.WithTimeout(ctx, refreshWait)
	defer cancel()

	held, release, err := t.Cache.Lock(wait, "integrations:refresh:"+name)
	if err != nil {
		return reread(ctx)
	}

	defer release()

	return renew(held)
}

/*
renewInstallation renews the installation's token once across every
process.
*/
func (t *Tokens) renewInstallation(ctx context.Context, row models.IntegrationInstallation) (string, error) {
	reread := func(ctx context.Context) (string, error) {
		_, creds, err := t.installation(ctx, row.ID)

		return creds.Access, err
	}

	return t.locked(ctx, row.ID.String(), reread, func(ctx context.Context) (string, error) {
		fresh, creds, err := t.installation(ctx, row.ID)
		if err != nil || !creds.expiring(t.Clock.Now(), t.Config.RefreshSkew) {
			return creds.Access, err
		}

		grant, err := t.OAuth.Refresh(ctx, fresh.Provider, creds.Refresh)
		if errors.Is(err, oauthclient.ErrInvalidGrant) {
			return "", t.broken(ctx, fresh)
		}

		if err != nil {
			return "", fmt.Errorf("refresh integration token: %w", err)
		}

		creds = creds.renewed(grant.Access, grant.Refresh)

		return creds.Access, t.storeInstallation(ctx, fresh, creds)
	})
}

/*
renewGrant renews the member's token once across every process.
*/
func (t *Tokens) renewGrant(ctx context.Context, row models.IntegrationInstallation, memberID ids.UUID,
) (string, error) {
	reread := func(ctx context.Context) (string, error) {
		_, creds, err := t.grant(ctx, row, memberID)

		return creds.Access, err
	}

	return t.locked(ctx, row.ID.String()+":"+memberID.String(), reread, func(ctx context.Context) (string, error) {
		grant, creds, err := t.grant(ctx, row, memberID)
		if err != nil || !creds.expiring(t.Clock.Now(), t.Config.RefreshSkew) {
			return creds.Access, err
		}

		renewed, err := t.OAuth.Refresh(ctx, row.Provider, creds.Refresh)
		if errors.Is(err, oauthclient.ErrInvalidGrant) {
			return "", t.brokenGrant(ctx, grant)
		}

		if err != nil {
			return "", fmt.Errorf("refresh integration member token: %w", err)
		}

		creds = creds.renewed(renewed.UserAccess, renewed.UserRefresh)

		return creds.Access, t.storeGrant(ctx, grant, creds)
	})
}

func (t *Tokens) storeInstallation(ctx context.Context, row models.IntegrationInstallation, c credentials) error {
	sealed, err := t.seal(ctx, row.WorkspaceID, installationAAD(row.ID), c)
	if err != nil {
		return err
	}

	return t.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		return wrap("store integration credentials", q.StoreIntegrationCredentials(ctx,
			models.StoreIntegrationCredentialsParams{
				WorkspaceID: row.WorkspaceID, ID: row.ID, CredentialsEncrypted: sealed, Now: t.Clock.Now(),
			}))
	})
}

func (t *Tokens) storeGrant(ctx context.Context, grant models.IntegrationMemberGrant, c credentials) error {
	sealed, err := t.seal(ctx, grant.WorkspaceID, grantAAD(grant.IntegrationInstallationID, grant.MemberID), c)
	if err != nil {
		return err
	}

	return t.inWorkspace(ctx, grant.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		return wrap("store integration member credentials", q.StoreIntegrationMemberCredentials(ctx,
			models.StoreIntegrationMemberCredentialsParams{
				WorkspaceID: grant.WorkspaceID, ID: grant.ID, CredentialsEncrypted: sealed, Now: t.Clock.Now(),
			}))
	})
}

/*
broken marks the installation broken because its renewal was refused,
and returns ErrIntegrationBroken.
*/
func (t *Tokens) broken(ctx context.Context, row models.IntegrationInstallation) error {
	err := t.fail(ctx, row, CodeRefreshInvalidGrant, "The provider refused to renew the token. Reconnect it.")
	if err != nil {
		return err
	}

	return ErrIntegrationBroken
}

/*
brokenGrant marks the member's grant failed because its renewal was
refused, and returns ErrNoMemberGrant.
*/
func (t *Tokens) brokenGrant(ctx context.Context, grant models.IntegrationMemberGrant) error {
	err := t.inWorkspace(ctx, grant.WorkspaceID, func(ctx context.Context, q *models.Queries, _ *database.Tx) error {
		return wrap("fail integration member grant", q.FailIntegrationMemberGrant(ctx,
			models.FailIntegrationMemberGrantParams{WorkspaceID: grant.WorkspaceID, ID: grant.ID, Now: t.Clock.Now()}))
	})
	if err != nil {
		return err
	}

	return ErrNoMemberGrant
}

/*
fail marks a live installation broken with the reason, and announces it
the first time.
*/
func (s *Services) fail(ctx context.Context, row models.IntegrationInstallation, code, message string) error {
	return s.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries, tx *database.Tx) error {
		failed, err := q.FailIntegrationInstallation(ctx, models.FailIntegrationInstallationParams{
			WorkspaceID: row.WorkspaceID, ID: row.ID, Code: code, Message: message, Now: s.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) || row.Status == StatusError {
			return nil
		}

		if err != nil {
			return fmt.Errorf("fail integration installation: %w", err)
		}

		return wrap("dispatch integration errored", s.Events.Dispatch(ctx, tx, appevents.IntegrationErrored{
			Base: s.base(failed.WorkspaceID, ids.Nil()), InstallationID: failed.ID, Provider: failed.Provider,
			Code: code,
		}))
	})
}
