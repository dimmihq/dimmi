/*
|--------------------------------------------------------------------------
| Update Auth Factor
|--------------------------------------------------------------------------
|
| A user names their factors so they can tell them apart, and picks the
| one the sign-in challenge offers first. Neither changes what protects
| the account, so neither needs a fresh password. A user has at most one
| preferred factor, moved in one transaction, and recovery codes are
| never it: they only stand in for another factor.
|
*/

package factors

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateAuthFactorInput is the user, their factor, the new name, nil to
keep it, whether to make the factor preferred, and the client.
*/
type UpdateAuthFactorInput struct {
	User      models.User
	FactorID  ids.UUID
	Name      *string
	Preferred bool
	Client    appauth.Client
}

/*
UpdateAuthFactor renames a factor or makes it the preferred one.

Rules:
  - Only the user's own factor is found, pending or active.
  - A name is cleaned of invisible characters and the rename is logged.
  - Making a factor preferred unsets the one preferred before.
  - Recovery codes cannot be preferred.

Errors: [exceptions.ErrNotFound], [exceptions.ErrPreferredNotAllowed],
or a wrapped error.
*/
type UpdateAuthFactor struct {
	DB          *database.DB
	Clock       clock.Clock
	SecurityLog *appauth.SecurityLog
}

/*
Handle applies the changes and returns the factor as it now is.
*/
func (a *UpdateAuthFactor) Handle(ctx context.Context, in UpdateAuthFactorInput) (models.UserAuthFactor, error) {
	var f models.UserAuthFactor

	err := a.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		q := models.New(a.DB)

		var err error

		f, err = q.LockUserAuthFactor(ctx, models.LockUserAuthFactorParams{ID: in.FactorID, UserID: in.User.ID})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("lock factor to update: %w", err)
		}

		if in.Preferred {
			if f, err = a.prefer(ctx, q, f); err != nil {
				return err
			}
		}

		if in.Name != nil {
			f, err = a.rename(ctx, in, f)
		}

		return err
	})

	return f, err
}

/*
prefer makes the factor the user's preferred one.
*/
func (a *UpdateAuthFactor) prefer(ctx context.Context, q *models.Queries, f models.UserAuthFactor) (
	models.UserAuthFactor, error,
) {
	if f.Type == factors.RecoveryCodeType {
		return f, exceptions.ErrPreferredNotAllowed
	}

	now := a.Clock.Now()

	err := q.UnpreferAuthFactors(ctx, models.UnpreferAuthFactorsParams{UserID: f.UserID, Now: now})
	if err != nil {
		return f, fmt.Errorf("unset preferred factor: %w", err)
	}

	f, err = q.PreferAuthFactor(ctx, models.PreferAuthFactorParams{ID: f.ID, Now: now})
	if err != nil {
		return f, fmt.Errorf("prefer factor: %w", err)
	}

	return f, nil
}

/*
rename gives the factor its new name and logs it.
*/
func (a *UpdateAuthFactor) rename(ctx context.Context, in UpdateAuthFactorInput, f models.UserAuthFactor) (
	models.UserAuthFactor, error,
) {
	f, err := models.New(a.DB).RenameAuthFactor(ctx, models.RenameAuthFactorParams{
		ID: f.ID, Name: str.Clean(*in.Name), Now: a.Clock.Now(),
	})
	if err != nil {
		return f, fmt.Errorf("rename factor: %w", err)
	}

	return f, a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventAuthFactorRenamed, Client: in.Client, Metadata: factorEvent(f),
	})
}
