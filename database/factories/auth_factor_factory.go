/*
|--------------------------------------------------------------------------
| Auth Factor Factory
|--------------------------------------------------------------------------
|
| Accounts protected by two-factor authentication, set up the way the
| account's owner ends up after enrolling: an active authenticator, a
| full set of recovery codes, and the account marked protected. The
| secret and the codes come back, so a test can answer the challenge the
| way a person with the phone and the printout would.
|
*/

package factories

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TwoFactorAccount is an account with two-factor authentication on: the
account, its authenticator and the base32 secret the app holds, and its
recovery set with the codes it was made with.
*/
type TwoFactorAccount struct {
	User          models.User
	TOTP          models.UserAuthFactor
	Secret        string
	Recovery      models.UserAuthFactor
	RecoveryCodes []string
}

/*
WithTwoFactor turns two-factor authentication on for the account with
an active authenticator and recovery codes.
*/
func WithTwoFactor(ctx context.Context, a *app.App, u models.User) (TwoFactorAccount, error) {
	account := TwoFactorAccount{User: u}

	totp, err := activeFactor(ctx, a, u, factors.TOTPType)
	if err != nil {
		return account, err
	}

	recovery, err := activeFactor(ctx, a, u, factors.RecoveryCodeType)
	if err != nil {
		return account, err
	}

	account.TOTP, account.Secret = totp.factor, totp.setup.Secret
	account.Recovery, account.RecoveryCodes = recovery.factor, recovery.setup.Codes
	account.User, err = EnableTwoFactor(ctx, a, u)

	return account, err
}

/*
madeFactor is a stored factor and what its owner saw once.
*/
type madeFactor struct {
	factor models.UserAuthFactor
	setup  factors.Setup
}

/*
activeFactor enrolls an active factor of the kind for the user, the
authenticator preferred.
*/
func activeFactor(ctx context.Context, a *app.App, u models.User, kind string) (madeFactor, error) {
	driver, _ := a.Factors.Known(kind)
	now := a.Clock.Now()
	id := ids.NewV7At(now)

	enrollment, err := driver.Enroll(ctx, u, id)
	if err != nil {
		return madeFactor{}, fmt.Errorf("enroll %s: %w", kind, err)
	}

	f, err := models.New(a.DB).CreateAuthFactor(ctx, models.CreateAuthFactorParams{
		ID: id, UserID: u.ID, Type: kind, Name: driver.DefaultName(), Status: string(enums.AuthFactorStatusActive),
		SecretEncrypted: enrollment.Secret, Credential: enrollment.Credential,
		IsPreferred: kind == factors.TOTPType, ConfirmedAt: &now, Now: now,
	})
	if err != nil {
		return madeFactor{}, fmt.Errorf("store %s: %w", kind, err)
	}

	return madeFactor{factor: f, setup: enrollment.Setup}, nil
}
