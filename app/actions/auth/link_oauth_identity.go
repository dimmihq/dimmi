/*
|--------------------------------------------------------------------------
| Link OAuth Identity
|--------------------------------------------------------------------------
|
| A signed-in user may add an account they hold at a provider as another
| way in. A provider account belongs to one user only, and a user links
| one account per provider, so a link never moves an account between
| users and never piles up. Every new link is logged and mailed to the
| owner, whoever made it, since it opens the account to whoever controls
| the provider's account.
|
*/

package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
LinkOAuthIdentityInput is the signed-in user, the provider account they
proved they hold, and the client.
*/
type LinkOAuthIdentityInput struct {
	UserID   ids.UUID
	Identity providers.ExternalIdentity
	Client   appauth.Client
}

/*
LinkOAuthIdentity links a provider account to a user.

Rules:
  - The user must still exist.
  - An account already linked to the user is only refreshed.
  - An account linked to another user, or a second account of the same
    provider, is refused.
  - A new link is logged and mailed to the user.

Errors: [exceptions.ErrNotFound] for a user that is gone,
[exceptions.ErrIdentityTaken], [exceptions.ErrProviderAlreadyLinked],
or a wrapped error.
*/
type LinkOAuthIdentity struct {
	DB          *database.DB
	Clock       clock.Clock
	Mail        *mail.Manager
	Links       appauth.Links
	Providers   providers.ProviderResolver
	SecurityLog *appauth.SecurityLog
}

/*
Handle links the account.
*/
func (a *LinkOAuthIdentity) Handle(ctx context.Context, in LinkOAuthIdentityInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, in.UserID)
		if errors.Is(err, database.ErrNotFound) || (err == nil && user.IsDeleted()) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("lock user to link identity: %w", err)
		}

		existing, err := q.FindOAuthIdentity(ctx, models.FindOAuthIdentityParams{
			Provider: in.Identity.Provider, ProviderUserID: in.Identity.Subject,
		})
		if err == nil && existing.UserID != user.ID {
			return exceptions.ErrIdentityTaken
		}

		if err == nil {
			return a.Touch(ctx, existing.ID, in.Identity, nil)
		}

		if !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find identity to link: %w", err)
		}

		return a.Attach(ctx, tx, user, in)
	})
}

/*
Attach links the provider account of the input to the user inside the
transaction, logs the link and mails the user about it.
*/
func (a *LinkOAuthIdentity) Attach(
	ctx context.Context, tx *database.Tx, user models.User, in LinkOAuthIdentityInput,
) error {
	identity := in.Identity
	if err := a.Create(ctx, user.ID, identity, nil); err != nil {
		return err
	}

	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventIdentityLinked, Client: in.Client,
		Metadata: map[string]any{"provider": identity.Provider},
	})
	if err != nil {
		return err
	}

	return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.IdentityLinkedMail{
		Name: user.Name, Provider: a.displayName(ctx, identity.Provider), SecurityURL: a.Links.Security(),
	})
}

/*
Create stores the link, inside the transaction in the context when
there is one. SignedInAt is when the account was last used to sign in,
nil for never.
*/
func (a *LinkOAuthIdentity) Create(
	ctx context.Context, userID ids.UUID, identity providers.ExternalIdentity, signedInAt *time.Time,
) error {
	_, err := models.New(a.DB).CreateOAuthIdentity(ctx, models.CreateOAuthIdentityParams{
		UserID: userID, Provider: identity.Provider, ProviderUserID: identity.Subject,
		Email: optional(identity.Email), EmailVerified: identity.EmailVerified, Username: optional(identity.Username),
		AvatarUrl: optional(identity.AvatarURL), HostedDomain: optional(identity.HostedDomain),
		LastLoginAt: signedInAt, Now: a.Clock.Now(),
	})

	switch {
	case database.IsUniqueViolation(err, "oauth_identities_provider_connection_subject_key"):
		return exceptions.ErrIdentityTaken
	case database.IsUniqueViolation(err, "oauth_identities_user_provider_connection_key"):
		return exceptions.ErrProviderAlreadyLinked
	case err != nil:
		return fmt.Errorf("create identity: %w", err)
	}

	return nil
}

/*
Touch refreshes what the provider reports about a linked account, and
records a sign-in with it when signedInAt is set, inside the
transaction in the context when there is one.
*/
func (a *LinkOAuthIdentity) Touch(
	ctx context.Context, id ids.UUID, identity providers.ExternalIdentity, signedInAt *time.Time,
) error {
	err := models.New(a.DB).TouchOAuthIdentity(ctx, models.TouchOAuthIdentityParams{
		ID: id, Email: optional(identity.Email), EmailVerified: identity.EmailVerified,
		Username: optional(identity.Username), AvatarUrl: optional(identity.AvatarURL),
		HostedDomain: optional(identity.HostedDomain), LastLoginAt: signedInAt, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("refresh identity: %w", err)
	}

	return nil
}

/*
displayName returns the provider's name as people know it, or its key
for a provider no longer configured.
*/
func (a *LinkOAuthIdentity) displayName(ctx context.Context, key string) string {
	name := key
	if provider, err := a.Providers.Find(ctx, key); err == nil {
		name = provider.Display().Name
	}

	return name
}

/*
optional returns nil for an empty value, so the column stays NULL when
the provider reported nothing.
*/
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
