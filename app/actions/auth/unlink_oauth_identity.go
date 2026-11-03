/*
|--------------------------------------------------------------------------
| Unlink OAuth Identity
|--------------------------------------------------------------------------
|
| A user may remove a provider account from theirs, but never the last
| way they have of signing in: a password, another linked provider the
| install still offers, or links by mail to a proven address. The
| account row is locked while this is decided, so two unlinks racing
| each other cannot both remove a way in that only one of them may.
|
*/

package auth

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
UnlinkOAuthIdentityInput is the user, the identity to unlink and the
client.
*/
type UnlinkOAuthIdentityInput struct {
	UserID     ids.UUID
	IdentityID ids.UUID
	Client     appauth.Client
}

/*
UnlinkOAuthIdentity removes one of the user's provider accounts.

Rules:
  - Only the user's own identities are found.
  - The user must keep a way to sign in: a password, another identity
    of a provider still offered, or magic links on with a proven email.
  - The unlink is logged.

Errors: [exceptions.ErrNotFound], [exceptions.ErrLastLoginMethod], or a
wrapped error.
*/
type UnlinkOAuthIdentity struct {
	DB          *database.DB
	Clock       clock.Clock
	Providers   providers.ProviderResolver
	MagicLink   config.MagicLinkConfig
	SecurityLog *appauth.SecurityLog
}

/*
Handle unlinks the identity.
*/
func (a *UnlinkOAuthIdentity) Handle(ctx context.Context, in UnlinkOAuthIdentityInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, in.UserID)
		if err != nil {
			return fmt.Errorf("lock user to unlink identity: %w", err)
		}

		identities, err := q.ListUserOAuthIdentities(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("list identities to unlink: %w", err)
		}

		target, others := split(identities, in.IdentityID)
		if target == nil {
			return exceptions.ErrNotFound
		}

		if !a.keepsWayIn(ctx, user, others) {
			return exceptions.ErrLastLoginMethod
		}

		_, err = q.DeleteOAuthIdentity(ctx, models.DeleteOAuthIdentityParams{ID: target.ID, UserID: user.ID})
		if err != nil {
			return fmt.Errorf("unlink identity: %w", err)
		}

		return a.SecurityLog.Record(ctx, appauth.Entry{
			UserID: user.ID, Type: appauth.EventIdentityUnlinked, Client: in.Client,
			Metadata: map[string]any{"provider": target.Provider},
		})
	})
}

/*
keepsWayIn reports whether the user can still sign in without the
identities left out of others.
*/
func (a *UnlinkOAuthIdentity) keepsWayIn(ctx context.Context, user models.User, others []models.OauthIdentity) bool {
	if user.HasPassword() || (a.MagicLink.Enabled && user.IsVerified()) {
		return true
	}

	for _, identity := range others {
		if _, err := a.Providers.Find(ctx, identity.Provider); err == nil {
			return true
		}
	}

	return false
}

/*
split returns the identity with the id, nil when there is none, and the
other identities.
*/
func split(identities []models.OauthIdentity, id ids.UUID) (*models.OauthIdentity, []models.OauthIdentity) {
	var (
		target *models.OauthIdentity
		others []models.OauthIdentity
	)

	for _, identity := range identities {
		if identity.ID == id {
			target = &identity

			continue
		}

		others = append(others, identity)
	}

	return target, others
}
