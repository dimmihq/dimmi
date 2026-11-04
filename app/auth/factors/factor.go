/*
|--------------------------------------------------------------------------
| Auth Factors
|--------------------------------------------------------------------------
|
| A factor is one way a user proves who they are beyond their first
| sign-in method. Each kind is a driver registered under its type, so a
| new kind is a new driver and no change to the sign-in flow or the
| table. A driver says what it can do through its capabilities, and the
| flows ask those instead of naming types.
|
| Some kinds answer with a code the person types, others with a signed
| credential from their device. A kind of the second sort hands the
| browser options before it can answer, at enrollment and at each
| challenge, and keeps state on the server until the answer comes.
|
| A factor's secret is sealed before it is stored, bound to the factor's
| own id, so a secret copied onto another row never opens.
|
*/

package factors

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Verification failures. ErrInvalidCode means the answer is wrong and
ErrCodeReused that it was right but already used, or came from a copy
of the factor; callers answer both the same way.
*/
var (
	ErrInvalidCode = twofactor.ErrInvalidCode
	ErrCodeReused  = twofactor.ErrCodeReused
)

/*
ErrUnknownFactor means the configuration enables a factor type no
driver is registered for, which stops the boot.
*/
var ErrUnknownFactor = errors.New("factors: unknown factor type")

/*
Capabilities say what a factor can do.

SecondFactor answers the challenge after a first sign-in. Backup marks a
factor that only stands in for another, so it never turns two-factor
authentication on alone. Primary signs in alone. Confirmation proves who
the user is before a sensitive change. PhishingResistant marks a factor
bound to the site it signs in to. Multiple lets a user hold several of
the kind. UserVisible lists the factor in the user's settings.
*/
type Capabilities struct {
	SecondFactor      bool
	Backup            bool
	Primary           bool
	Confirmation      bool
	PhishingResistant bool
	Multiple          bool
	UserVisible       bool
}

/*
Setup is what a user sees once, while enrolling: for an authenticator
app the secret to type in, the otpauth URL and its QR code; for
recovery codes the codes; for a passkey the options the browser
creates it with.
*/
type Setup struct {
	Secret     string
	OTPAuthURL string
	QRCodeSVG  string
	Codes      []string
	Options    json.RawMessage
}

/*
EnrollInput is the user a new factor is for, the id it will be stored
under, and the factors the user already holds.
*/
type EnrollInput struct {
	User models.User
	ID   ids.UUID
	Held []models.UserAuthFactor
}

/*
Enrollment is a new factor to store: its sealed secret, the non-secret
data the driver keeps, the status it starts in, and what to show the
user once.
*/
type Enrollment struct {
	Secret     encryption.Sealed
	Credential json.RawMessage
	Status     enums.AuthFactorStatus
	Setup      Setup
}

/*
Factor is one kind of auth factor.
*/
type Factor interface {
	/*
		Type returns the kind's name, such as "totp".
	*/
	Type() string

	/*
		DefaultName returns the name a new factor of the kind gets when the
		user gives none.
	*/
	DefaultName() string

	/*
		Capabilities returns what the kind can do.
	*/
	Capabilities() Capabilities

	/*
		Enroll starts a new factor for the user under the id.
	*/
	Enroll(ctx context.Context, in EnrollInput) (Enrollment, error)

	/*
		Verify checks the answer against the factor and updates the stored
		state that checking changes, such as the last step used. It returns
		ErrInvalidCode or ErrCodeReused when the answer does not pass.
	*/
	Verify(ctx context.Context, factor *models.UserAuthFactor, answer Answer) error
}

/*
EnablesTwoFactor reports whether an active factor of the kind turns
two-factor authentication on: it answers the challenge and is not only
a stand-in for another factor.
*/
func EnablesTwoFactor(f Factor) bool {
	c := f.Capabilities()

	return c.SecondFactor && !c.Backup
}
