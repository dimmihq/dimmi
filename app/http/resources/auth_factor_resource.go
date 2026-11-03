/*
|--------------------------------------------------------------------------
| Auth Factor Resources
|--------------------------------------------------------------------------
|
| A user's factor as their settings show it: its kind, name and state,
| and for recovery codes how many are left. Secrets never leave the
| server, except once, right after they are made: the setup of a new
| authenticator and the codes of a new recovery set ride on the answer
| that made them, which no cache may keep.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/auth/factors"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AuthFactor is one of the user's factors. Remaining and Total are set on
recovery codes only. Setup, Codes and RecoveryCodes are set only on the
answer that made them: the authenticator's setup, a new recovery set's
codes, and the recovery set made when an activation turned two-factor
authentication on.
*/
type AuthFactor struct {
	ID            string      `json:"id"`
	Object        string      `json:"object"`
	Type          string      `json:"type"`
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	Preferred     bool        `json:"preferred"`
	Remaining     *int        `json:"remaining,omitempty"`
	Total         *int        `json:"total,omitempty"`
	ConfirmedAt   *time.Time  `json:"confirmed_at"`
	LastUsedAt    *time.Time  `json:"last_used_at"`
	CreatedAt     time.Time   `json:"created_at"`
	Setup         *TOTPSetup  `json:"setup,omitempty"`
	Codes         []string    `json:"codes,omitempty"`
	RecoveryCodes *AuthFactor `json:"recovery_codes,omitempty"`
}

/*
TOTPSetup is what a user needs to add a new authenticator to their app:
the secret to type in, the otpauth URL, and the URL as a QR code.
*/
type TOTPSetup struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
	QRSVG      string `json:"qr_svg"`
}

/*
NewAuthFactor shapes a factor.
*/
func NewAuthFactor(f models.UserAuthFactor) AuthFactor {
	out := AuthFactor{
		ID:          models.PublicID(models.PrefixAuthFactor, f.ID),
		Object:      "auth_factor",
		Type:        f.Type,
		Name:        f.Name,
		Status:      f.Status,
		Preferred:   f.IsPreferred,
		ConfirmedAt: utc(f.ConfirmedAt),
		LastUsedAt:  utc(f.LastUsedAt),
		CreatedAt:   f.CreatedAt.UTC(),
	}

	if f.Type == factors.RecoveryCodeType {
		codes := factors.CodesOf(f)
		out.Remaining, out.Total = &codes.Remaining, &codes.Total
	}

	return out
}

/*
NewAuthFactors shapes the factors, in the order given.
*/
func NewAuthFactors(held []models.UserAuthFactor) []AuthFactor {
	out := make([]AuthFactor, 0, len(held))
	for _, f := range held {
		out = append(out, NewAuthFactor(f))
	}

	return out
}

/*
NewMadeAuthFactor shapes a factor just made, with what the user sees of
it once.
*/
func NewMadeAuthFactor(f models.UserAuthFactor, setup factors.Setup) AuthFactor {
	out := NewAuthFactor(f)
	out.Codes = setup.Codes

	if setup.Secret != "" {
		out.Setup = &TOTPSetup{Secret: setup.Secret, OTPAuthURL: setup.OTPAuthURL, QRSVG: setup.QRCodeSVG}
	}

	return out
}

/*
utc returns the time in UTC, or nil.
*/
func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}

	at := t.UTC()

	return &at
}
