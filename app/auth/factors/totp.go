/*
|--------------------------------------------------------------------------
| Authenticator App Factor
|--------------------------------------------------------------------------
|
| Six-digit codes from an authenticator app, the settings every app
| understands: SHA-1, six digits, thirty seconds, one step of drift
| either way. A code is never accepted twice: the factor keeps the last
| step it accepted, and the step is also claimed in the shared cache.
|
| A user holds one authenticator at a time; replacing a phone means
| removing the old one first. Enrolling starts pending, and only a code
| from the app proves the user can produce codes before it counts.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TOTPType is the type authenticator app factors are stored under.
*/
const TOTPType = twofactor.TOTPType

/*
totpSettings is the non-secret data kept with an authenticator, so a
reader of the table knows how its codes are made.
*/
const totpSettings = `{"algorithm":"SHA1","digits":6,"period":30}`

/*
TOTP is the authenticator app driver. Issuer names the install in the
apps, Keys seal the seeds, Cache holds the steps already used.
*/
type TOTP struct {
	Issuer string
	Keys   encryption.KeyManager
	Cache  *cache.Store
	Clock  clock.Clock
}

var _ Factor = (*TOTP)(nil)

/*
Type returns TOTPType.
*/
func (d *TOTP) Type() string {
	return TOTPType
}

/*
DefaultName returns the name a new authenticator gets.
*/
func (d *TOTP) DefaultName() string {
	return "Authenticator app"
}

/*
Capabilities says an authenticator answers the challenge, confirms
sensitive changes and is listed, one per user.
*/
func (d *TOTP) Capabilities() Capabilities {
	return Capabilities{SecondFactor: true, Confirmation: true, UserVisible: true}
}

/*
Enroll makes a new seed for the factor and what the user needs to add
it to their app.
*/
func (d *TOTP) Enroll(ctx context.Context, user models.User, id ids.UUID) (Enrollment, error) {
	codes := d.codes(id)

	sealed, err := codes.NewSecret(ctx)
	if err != nil {
		return Enrollment{}, fmt.Errorf("make authenticator seed: %w", err)
	}

	secret, url, err := codes.Setup(ctx, sealed, user.Email)
	if err != nil {
		return Enrollment{}, fmt.Errorf("describe authenticator seed: %w", err)
	}

	qr, err := twofactor.QRCodeSVG(url)
	if err != nil {
		return Enrollment{}, fmt.Errorf("draw authenticator qr code: %w", err)
	}

	return Enrollment{
		Secret:     sealed,
		Credential: []byte(totpSettings),
		Status:     enums.AuthFactorStatusPending,
		Setup:      Setup{Secret: secret, OTPAuthURL: url, QRCodeSVG: qr},
	}, nil
}

/*
Verify accepts a code for a step after the last one used, and records
the step.
*/
func (d *TOTP) Verify(ctx context.Context, factor *models.UserAuthFactor, input string) error {
	cred := twofactor.Credential{ID: factor.ID, Type: factor.Type, Secret: factor.SecretEncrypted}
	if factor.LastCounter != nil {
		cred.Counter = *factor.LastCounter
	}

	if err := d.codes(factor.ID).Verify(ctx, &cred, input); err != nil {
		return err
	}

	factor.LastCounter = &cred.Counter

	return nil
}

/*
codes returns the one-time password maker for the factor, sealing with
its id.
*/
func (d *TOTP) codes(id ids.UUID) *twofactor.TOTP {
	return twofactor.NewTOTP(d.Issuer, bind(d.Keys, id), d.Cache, d.Clock)
}
