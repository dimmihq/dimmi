/*
|--------------------------------------------------------------------------
| Two-Factor Mail
|--------------------------------------------------------------------------
|
| Every change to how much a sign-in must prove is told to the account's
| owner: two-factor authentication turned on or off, and a recovery code
| spent, which means the authenticator was not at hand, or was not
| theirs to begin with.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
TwoFactorEnabledMail tells a user their account now asks for a second
factor, and links their security settings.
*/
type TwoFactorEnabledMail struct {
	Name        string `json:"name"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TwoFactorEnabledMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TwoFactorEnabledMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.two_factor_enabled.subject", accounts.TwoFactorEnabled(m.Name, m.SecurityURL)), nil
}

/*
TwoFactorDisabledMail tells a user their account no longer asks for a
second factor, and links their security settings.
*/
type TwoFactorDisabledMail struct {
	Name        string `json:"name"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (TwoFactorDisabledMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m TwoFactorDisabledMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.two_factor_disabled.subject", accounts.TwoFactorDisabled(m.Name, m.SecurityURL)), nil
}

/*
RecoveryCodeUsedMail tells a user a recovery code signed them in and
how many codes Remaining, and links their security settings.
*/
type RecoveryCodeUsedMail struct {
	Name        string `json:"name"`
	Remaining   int    `json:"remaining"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (RecoveryCodeUsedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m RecoveryCodeUsedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.recovery_code_used.subject",
		accounts.RecoveryCodeUsed(m.Name, m.Remaining, m.SecurityURL)), nil
}
