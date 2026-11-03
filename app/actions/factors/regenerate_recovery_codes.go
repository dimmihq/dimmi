/*
|--------------------------------------------------------------------------
| Regenerate Recovery Codes
|--------------------------------------------------------------------------
|
| A user who used up or lost their recovery codes makes a new set. The
| old codes stop working the moment the new ones exist, and the new ones
| are shown once.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RegenerateRecoveryCodesInput is the user and the client.
*/
type RegenerateRecoveryCodesInput struct {
	User   models.User
	Client appauth.Client
}

/*
RegenerateRecoveryCodes replaces the user's recovery codes.

Rules:
  - Only an account with two-factor authentication on gets codes.
  - The old set is deleted and the new one stored in one transaction.
  - The change is logged.

Errors: [exceptions.ErrTwoFactorNotEnabled],
[exceptions.ErrFactorTypeNotAvailable] while recovery codes are
disabled, or a wrapped error.
*/
type RegenerateRecoveryCodes struct {
	DB          *database.DB
	Clock       clock.Clock
	Factors     *factors.Registry
	SecurityLog *appauth.SecurityLog
}

/*
Handle stores the new set and returns it with its codes.
*/
func (a *RegenerateRecoveryCodes) Handle(ctx context.Context, in RegenerateRecoveryCodesInput) (
	NewAuthFactor, error,
) {
	driver, ok := a.Factors.Find(factors.RecoveryCodeType)
	if !ok {
		return NewAuthFactor{}, exceptions.ErrFactorTypeNotAvailable
	}

	var set NewAuthFactor

	err := a.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		q := models.New(a.DB)

		user, err := q.LockUser(ctx, in.User.ID)
		if err != nil {
			return fmt.Errorf("lock user to regenerate recovery codes: %w", err)
		}

		if !user.TwoFactorEnabled() {
			return exceptions.ErrTwoFactorNotEnabled
		}

		set, err = writer{q: q, now: a.Clock.Now()}.replaceRecoveryCodes(ctx, driver, user)
		if err != nil {
			return err
		}

		return a.SecurityLog.Record(ctx, appauth.Entry{
			UserID: user.ID, Type: appauth.EventAuthFactorRecoveryCodesRenewed, Client: in.Client,
			Metadata: factorEvent(set.Factor),
		})
	})

	return set, err
}
