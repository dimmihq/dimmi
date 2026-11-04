/*
|--------------------------------------------------------------------------
| Remove Auth Factor
|--------------------------------------------------------------------------
|
| Removing one factor. An unfinished one simply goes. Removing the last
| factor that protects the account turns two-factor authentication off,
| with everything that entails. Recovery codes alone cannot be removed
| while the account stays protected: they are the way back in when the
| authenticator is lost, so the user makes new ones instead.
|
| When the preferred factor goes and others stay, the oldest of those
| that protect the account becomes the preferred one.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RemoveAuthFactorInput is the user, their factor, the client and the
first-party app grant to keep should two-factor authentication turn
off, ids.Nil() for none.
*/
type RemoveAuthFactorInput struct {
	User      models.User
	FactorID  ids.UUID
	Client    appauth.Client
	KeepGrant ids.UUID
}

/*
RemoveAuthFactor removes one of the user's factors.

Rules:
  - Only the user's own factor is found.
  - A pending factor is deleted without anything else.
  - Recovery codes cannot be removed while two-factor authentication
    is on.
  - Removing the last factor that turns two-factor authentication on
    turns it off through [DisableTwoFactor].
  - Otherwise the factor is deleted and logged, and a new preferred
    factor is picked when it was the preferred one.

Errors: [exceptions.ErrNotFound], [exceptions.ErrRecoveryCodesRequired],
those of [DisableTwoFactor], or a wrapped error.
*/
type RemoveAuthFactor struct {
	DB          *database.DB
	Clock       clock.Clock
	Factors     *factors.Registry
	Disable     *DisableTwoFactor
	SecurityLog *appauth.SecurityLog
}

/*
Handle removes the factor.
*/
func (a *RemoveAuthFactor) Handle(ctx context.Context, in RemoveAuthFactorInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		user, err := q.LockUser(ctx, in.User.ID)
		if err != nil {
			return fmt.Errorf("lock user to remove factor: %w", err)
		}

		held, err := q.LockUserAuthFactors(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("lock factors to remove one: %w", err)
		}

		target, others := splitFactors(held, in.FactorID)

		switch {
		case target == nil:
			return exceptions.ErrNotFound
		case target.Status == string(enums.AuthFactorStatusPending):
			return a.delete(ctx, q, *target)
		case target.Type == factors.RecoveryCodeType && user.TwoFactorEnabled():
			return exceptions.ErrRecoveryCodesRequired
		case a.Factors.EnablesTwoFactor(target.Type) && !a.protectedBy(others):
			in.User = user

			return a.disable(ctx, tx, in, *target)
		}

		if err := a.delete(ctx, q, *target); err != nil {
			return err
		}

		if err := a.reprefer(ctx, q, *target, others); err != nil {
			return err
		}

		return a.SecurityLog.Record(ctx, appauth.Entry{
			UserID: user.ID, Type: appauth.EventAuthFactorRemoved, Client: in.Client, Metadata: factorEvent(*target),
		})
	})
}

/*
protectedBy reports whether one of the factors is active and turns
two-factor authentication on.
*/
func (a *RemoveAuthFactor) protectedBy(held []models.UserAuthFactor) bool {
	for _, f := range held {
		if f.Status == string(enums.AuthFactorStatusActive) && a.Factors.EnablesTwoFactor(f.Type) {
			return true
		}
	}

	return false
}

/*
disable logs the removal of the last protecting factor and turns
two-factor authentication off for the locked user of the input. A
workspace requiring a second factor refuses it before anything is
logged.
*/
func (a *RemoveAuthFactor) disable(
	ctx context.Context, tx *database.Tx, in RemoveAuthFactorInput, target models.UserAuthFactor,
) error {
	if err := blockedByWorkspaces(ctx, models.New(tx), in.User); err != nil {
		return err
	}

	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventAuthFactorRemoved, Client: in.Client, Metadata: factorEvent(target),
	})
	if err != nil {
		return err
	}

	return a.Disable.Handle(ctx, tx, DisableTwoFactorInput{User: in.User, Client: in.Client, KeepGrant: in.KeepGrant})
}

/*
delete deletes the factor.
*/
func (a *RemoveAuthFactor) delete(ctx context.Context, q *models.Queries, f models.UserAuthFactor) error {
	if err := q.DeleteAuthFactor(ctx, models.DeleteAuthFactorParams{ID: f.ID, UserID: f.UserID}); err != nil {
		return fmt.Errorf("delete factor: %w", err)
	}

	return nil
}

/*
reprefer makes the oldest remaining protecting factor preferred when
the removed one was.
*/
func (a *RemoveAuthFactor) reprefer(
	ctx context.Context, q *models.Queries, removed models.UserAuthFactor, others []models.UserAuthFactor,
) error {
	if !removed.IsPreferred {
		return nil
	}

	for _, f := range others {
		if f.Status != string(enums.AuthFactorStatusActive) || !a.Factors.EnablesTwoFactor(f.Type) {
			continue
		}

		if _, err := q.PreferAuthFactor(ctx, models.PreferAuthFactorParams{ID: f.ID, Now: a.Clock.Now()}); err != nil {
			return fmt.Errorf("prefer next factor: %w", err)
		}

		return nil
	}

	return nil
}

/*
splitFactors returns the factor with the id, nil when there is none, and
the other factors.
*/
func splitFactors(held []models.UserAuthFactor, id ids.UUID) (*models.UserAuthFactor, []models.UserAuthFactor) {
	var (
		target *models.UserAuthFactor
		others []models.UserAuthFactor
	)

	for _, f := range held {
		if f.ID == id {
			target = &f

			continue
		}

		others = append(others, f)
	}

	return target, others
}
