/*
|--------------------------------------------------------------------------
| Password Changed Mail
|--------------------------------------------------------------------------
|
| Every change to how a user signs in with a password is told to them,
| so a change they did not make never goes unnoticed: a new password, a
| reset, or a password removed to protect a claimed account.
|
*/

package mail

import (
	"context"

	"github.com/dimmihq/dimmi/framework/mail"

	"github.com/dimmihq/dimmi/resources/views/mail/accounts"
)

/*
Why a password changed, as PasswordChangedMail tells it.
*/
const (
	PasswordWasChanged = "changed"
	PasswordWasReset   = "reset"
	PasswordWasRemoved = "removed"
)

/*
PasswordChangedMail tells a user their password changed for Reason, one
of the reasons above, and links their security settings.
*/
type PasswordChangedMail struct {
	Name        string `json:"name"`
	Reason      string `json:"reason"`
	SecurityURL string `json:"security_url"`
}

/*
Envelope leaves the subject to the content, which translates it.
*/
func (PasswordChangedMail) Envelope() mail.Envelope {
	return mail.Envelope{}
}

/*
Content renders the mail.
*/
func (m PasswordChangedMail) Content(ctx context.Context) (mail.Content, error) {
	return content(ctx, "mail.password_changed.subject", accounts.PasswordChanged(m.Name, m.Reason, m.SecurityURL)), nil
}
