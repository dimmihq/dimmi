/*
|--------------------------------------------------------------------------
| Add Auth Factor
|--------------------------------------------------------------------------
|
| Adding a factor starts its enrollment: the secret is made and stored,
| pending, and shown to the user once, so they can add it to their app.
| A pending factor counts nowhere until a code activates it. Starting
| over replaces the earlier pending factor of the same kind, so two open
| tabs never leave two half-made authenticators behind.
|
| Asking for recovery codes instead makes a new set straight away, which
| only an account with two-factor authentication on has any use for.
|
*/

package factors

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/support/str"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
AddAuthFactorInput is the user, the kind of factor and its name, empty
for the kind's default, and the client.
*/
type AddAuthFactorInput struct {
	User   models.User
	Type   string
	Name   string
	Client appauth.Client
}

/*
AddAuthFactor starts a new factor for the user.

Rules:
  - At most ten factors a user may add per hour.
  - Only enabled kinds may be added.
  - A kind held once may not be added again while one is active, and
    active factors never exceed the install's limit.
  - An earlier pending factor of the kind is replaced.
  - Recovery codes are made anew through [RegenerateRecoveryCodes].

Errors: [exceptions.ErrFactorTypeNotAvailable],
[exceptions.ErrFactorLimitReached], [exceptions.ErrTooManyAttempts],
those of [RegenerateRecoveryCodes], or a wrapped error.
*/
type AddAuthFactor struct {
	DB         *database.DB
	Clock      clock.Clock
	Limiter    *ratelimit.Limiter
	Factors    *factors.Registry
	Config     config.FactorsConfig
	Regenerate *RegenerateRecoveryCodes
}

/*
Handle stores the pending factor and returns it with its setup.
*/
func (a *AddAuthFactor) Handle(ctx context.Context, in AddAuthFactorInput) (NewAuthFactor, error) {
	driver, ok := a.Factors.Find(in.Type)
	if !ok {
		return NewAuthFactor{}, exceptions.ErrFactorTypeNotAvailable
	}

	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "factors:add:" + in.User.ID.String(), Limit: 10, Window: time.Hour,
	})
	if err != nil {
		return NewAuthFactor{}, err
	}

	if driver.Type() == factors.RecoveryCodeType {
		return a.Regenerate.Handle(ctx, RegenerateRecoveryCodesInput{User: in.User, Client: in.Client})
	}

	var added NewAuthFactor

	err = a.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		q := models.New(a.DB)

		if _, err := q.LockUser(ctx, in.User.ID); err != nil {
			return fmt.Errorf("lock user to add factor: %w", err)
		}

		held, err := q.LockUserAuthFactors(ctx, in.User.ID)
		if err != nil {
			return fmt.Errorf("lock factors to add one: %w", err)
		}

		if !a.allows(driver, held) {
			return exceptions.ErrFactorLimitReached
		}

		err = q.DeleteAuthFactorsOfType(ctx, models.DeleteAuthFactorsOfTypeParams{
			UserID: in.User.ID, Type: driver.Type(), Status: string(enums.AuthFactorStatusPending),
		})
		if err != nil {
			return fmt.Errorf("replace pending factor: %w", err)
		}

		added, err = writer{q: q, now: a.Clock.Now()}.store(ctx, driver, in.User, str.Clean(in.Name))

		return err
	})

	return added, err
}

/*
allows reports whether the user may hold another factor of the kind
beside the ones they hold.
*/
func (a *AddAuthFactor) allows(driver factors.Factor, held []models.UserAuthFactor) bool {
	active := 0

	for _, f := range held {
		if f.Status != string(enums.AuthFactorStatusActive) {
			continue
		}

		active++

		if f.Type == driver.Type() && !driver.Capabilities().Multiple {
			return false
		}
	}

	return active < a.Config.MaxPerUser
}
